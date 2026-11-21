package api_test

import (
 "bytes"
 "context"
 "encoding/json"
 "fmt"
 "io"
 "log/slog"
 "net/http"
 "net/http/httptest"
 "os"
 "testing"
 "time"

 "github.com/Folsik/Property-Management-Inspections/internal/api"
 "github.com/Folsik/Property-Management-Inspections/internal/auth"
 "github.com/Folsik/Property-Management-Inspections/internal/store"
)

func TestStage4AuthenticatedJobAndPDFIsolation(t *testing.T) {
 if os.Getenv("RUN_DB_INTEGRATION")!="1" { t.Skip("PostgreSQL integration only") }
 ctx,cancel:=context.WithTimeout(context.Background(),70*time.Second)
 defer cancel()
 db,err:=store.Open(ctx,os.Getenv("DATABASE_URL"));if err!=nil{t.Fatal(err)}
 defer db.Close()
 if err=db.Migrate(ctx);err!=nil{t.Fatal(err)}
 unique:=time.Now().UnixNano()%10000000000
 var owner,outsider,org,property,inspection,media string
 for i,id:=range []*string{&owner,&outsider} {
  if err=db.DB.QueryRow(ctx,"INSERT INTO users(phone) VALUES($1) RETURNING id",fmt.Sprintf("+1555%010d",unique+int64(i))).Scan(id);err!=nil{t.Fatal(err)}
 }
 defer db.DB.Exec(context.Background(),"DELETE FROM users WHERE id=$1 OR id=$2",owner,outsider)
 if err=db.DB.QueryRow(ctx,"INSERT INTO organizations(name,created_by) VALUES('Stage4 test', $1) RETURNING id",owner).Scan(&org);err!=nil{t.Fatal(err)}
 defer db.DB.Exec(context.Background(),"DELETE FROM organizations WHERE id=$1",org)
 if _,err=db.DB.Exec(ctx,"INSERT INTO organization_members(organization_id,user_id,role) VALUES($1,$2,'owner')",org,owner);err!=nil{t.Fatal(err)}
 if err=db.DB.QueryRow(ctx,"INSERT INTO properties(organization_id,name) VALUES($1,'Test property') RETURNING id",org).Scan(&property);err!=nil{t.Fatal(err)}
 if err=db.DB.QueryRow(ctx,"INSERT INTO inspections(organization_id,property_id,created_by,inspection_type) VALUES($1,$2,$3,'maintenance') RETURNING id",org,property,owner).Scan(&inspection);err!=nil{t.Fatal(err)}
 if err=db.DB.QueryRow(ctx,`INSERT INTO media_attachments(organization_id,inspection_id,uploaded_by,object_key,mime_type,size_bytes,state,verified_at)
 VALUES($1,$2,$3,$4,'audio/mp4',10,'uploaded',now()) RETURNING id`,
 org,inspection,owner,"test-audio-"+fmt.Sprint(unique)).Scan(&media);err!=nil{t.Fatal(err)}
 tokens,err:=auth.NewManager("stage4-audit",15*time.Minute);if err!=nil{t.Fatal(err)}
 ownerToken,_,err:=tokens.Issue(owner);if err!=nil{t.Fatal(err)}
 outsiderToken,_,err:=tokens.Issue(outsider);if err!=nil{t.Fatal(err)}
 server:=httptest.NewServer(api.New(db.DB,tokens,slog.New(slog.NewTextHandler(io.Discard,nil)),"development").Handler())
 defer server.Close()
 call:=func(method,path,token string,body any,want int)[]byte {
  t.Helper()
  var input io.Reader
  if body!=nil{data,e:=json.Marshal(body);if e!=nil{t.Fatal(e)};input=bytes.NewReader(data)}
  req,e:=http.NewRequestWithContext(ctx,method,server.URL+path,input);if e!=nil{t.Fatal(e)}
  req.Header.Set("Authorization","Bearer "+token)
  req.Header.Set("Content-Type","application/json")
  res,e:=http.DefaultClient.Do(req);if e!=nil{t.Fatal(e)}
  defer res.Body.Close()
  payload,e:=io.ReadAll(io.LimitReader(res.Body,2<<20));if e!=nil{t.Fatal(e)}
  if res.StatusCode!=want{t.Fatalf("%s %s: want %d got %d: %s",method,path,want,res.StatusCode,payload)}
  return payload
 }
 prefix:="/v1/organizations/"+org+"/inspections/"+inspection
 key:="2b3d9ae0-7f34-47fa-aac4-bc842b779d00"
 request:=map[string]string{"media_id":media,"idempotency_key":key}
 call("POST",prefix+"/transcriptions",outsiderToken,request,403)
 var first map[string]string
 if err=json.Unmarshal(call("POST",prefix+"/transcriptions",ownerToken,request,202),&first);err!=nil{t.Fatal(err)}
 var repeated map[string]string
 if err=json.Unmarshal(call("POST",prefix+"/transcriptions",ownerToken,request,202),&repeated);err!=nil{t.Fatal(err)}
 if first["id"]==""||first["id"]!=repeated["id"]{t.Fatalf("duplicate job: %v %v",first,repeated)}
 var jobs int
 if err=db.DB.QueryRow(ctx,"SELECT count(*) FROM jobs WHERE organization_id=$1 AND kind='transcribe' AND idempotency_key=$2",org,key).Scan(&jobs);err!=nil||jobs!=1{t.Fatalf("job count %d %v",jobs,err)}
 if _,err=db.DB.Exec(ctx,"INSERT INTO inspection_transcripts(organization_id,inspection_id,media_id,text) VALUES($1,$2,$3,'Test note')",org,inspection,media);err!=nil{t.Fatal(err)}
 if _,err=db.DB.Exec(ctx,"INSERT INTO inspection_generated_reports(organization_id,inspection_id,media_id,content) VALUES($1,$2,$3,$4)",org,inspection,media,[]byte("%PDF-1.4\nhello"));err!=nil{t.Fatal(err)}
 pdfPath:=prefix+"/reports/"+media+"/pdf"
 call("GET",pdfPath,outsiderToken,nil,403)
 pdf:=call("GET",pdfPath,ownerToken,nil,200)
 if !bytes.HasPrefix(pdf,[]byte("%PDF-1.4")){t.Fatalf("PDF bytes corrupted: %q",pdf)}
 if _,err=db.DB.Exec(ctx,"UPDATE media_attachments SET state='deleted',deleted_at=now() WHERE id=$1",media);err!=nil{t.Fatal(err)}
 call("GET",pdfPath,ownerToken,nil,404)
}
