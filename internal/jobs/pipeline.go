package jobs

import (
 "bytes"
 "context"
 "encoding/base64"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "log/slog"
 "net/http"
 "net/url"
 "os"
 "strings"
 "time"
 "unicode"

 "github.com/Folsik/Property-Management-Inspections/internal/api"
 "github.com/jackc/pgx/v5/pgxpool"
)
type Pipeline struct {
 DB *pgxpool.Pool
 Media *api.Server
 Client *http.Client
 Logger *slog.Logger
}
type task struct {
 InspectionID string `json:"inspection_id"`
 MediaID string `json:"media_id"`
}
func parseTask(j Job)(task,error) {
 var t task
 if err:=json.Unmarshal(j.Payload,&t);err!=nil{return t,err}
 if t.InspectionID==""||t.MediaID==""{return t,errors.New("missing inspection/media identifier")}
 return t,nil
}
func (p *Pipeline) Handlers() map[string]Handler {
 return map[string]Handler{"transcribe":p.Transcribe,"extract":p.Extract,"generate_pdf":p.GeneratePDF}
}
func (p *Pipeline) Transcribe(ctx context.Context,j Job)(json.RawMessage,error) {
 t,err:=parseTask(j);if err!=nil{return nil,err}
 endpoint:=os.Getenv("TRANSCRIPTION_ENDPOINT")
 secret:=os.Getenv("TRANSCRIPTION_API_KEY")
 u,err:=url.Parse(endpoint)
 if err!=nil||u.Scheme!="https"||u.Host==""||u.User!=nil||u.Fragment!=""||secret==""{
  return nil,errors.New("HTTPS transcription endpoint and API key must be configured")
 }
 audio,mime,err:=p.Media.DownloadWorkerAudio(ctx,j.OrganizationID,t.InspectionID,t.MediaID)
 if err!=nil{return nil,fmt.Errorf("read verified audio: %w",err)}
 requestBody,err:=json.Marshal(map[string]any{"audio_base64":base64.StdEncoding.EncodeToString(audio),"mime_type":mime})
 if err!=nil{return nil,err}
 req,err:=http.NewRequestWithContext(ctx,"POST",u.String(),bytes.NewReader(requestBody))
 if err!=nil{return nil,err}
 req.Header.Set("Authorization","Bearer "+secret)
 req.Header.Set("Content-Type","application/json")
 client:=p.Client
 if client==nil{client=&http.Client{Timeout:3*time.Minute}}
 res,err:=client.Do(req)
 if err!=nil{return nil,err}
 defer res.Body.Close()
 if res.StatusCode!=200 {
  return nil,fmt.Errorf("transcription service HTTP %d",res.StatusCode)
 }
 var response struct{Text string `json:"text"`}
 if err:=json.NewDecoder(io.LimitReader(res.Body,131072)).Decode(&response);err!=nil{return nil,err}
 response.Text=strings.TrimSpace(response.Text)
 if response.Text==""||len(response.Text)>65536{return nil,errors.New("empty or oversized transcription")}
 _,err=p.DB.Exec(ctx,`INSERT INTO inspection_transcripts(organization_id,inspection_id,media_id,text)
 VALUES($1,$2,$3,$4) ON CONFLICT (organization_id,inspection_id,media_id)
 DO UPDATE SET text=EXCLUDED.text,updated_at=now()`,j.OrganizationID,t.InspectionID,t.MediaID,response.Text)
 if err!=nil{return nil,err}
 return json.Marshal(map[string]any{"characters":len(response.Text)})
}
func (p *Pipeline) Extract(ctx context.Context,j Job)(json.RawMessage,error){
 t,err:=parseTask(j);if err!=nil{return nil,err}
 var transcript string
 err=p.DB.QueryRow(ctx,`SELECT text FROM inspection_transcripts WHERE organization_id=$1 AND inspection_id=$2 AND media_id=$3`,j.OrganizationID,t.InspectionID,t.MediaID).Scan(&transcript)
 if err!=nil{return nil,err}
 // Deterministic, non-AI extraction: retains source wording and never invents findings.
 chunks:=strings.FieldsFunc(transcript,func(r rune)bool{return r=='\n'||r=='\r'})
 notes:=make([]string,0,32)
 for _,chunk:=range chunks {
  chunk=strings.TrimSpace(chunk)
  if chunk!=""{notes=append(notes,chunk)}
  if len(notes)==100{break}
 }
 data,err:=json.Marshal(notes);if err!=nil{return nil,err}
 _,err=p.DB.Exec(ctx,`INSERT INTO inspection_extractions(organization_id,inspection_id,media_id,notes)
 VALUES($1,$2,$3,$4) ON CONFLICT (organization_id,inspection_id,media_id)
 DO UPDATE SET notes=EXCLUDED.notes,updated_at=now()`,j.OrganizationID,t.InspectionID,t.MediaID,string(data))
 if err!=nil{return nil,err}
 return json.Marshal(map[string]any{"notes":len(notes),"mode":"verbatim_lines"})
}
func (p *Pipeline) GeneratePDF(ctx context.Context,j Job)(json.RawMessage,error){
 t,err:=parseTask(j);if err!=nil{return nil,err}
 var propertyName,transcript string
 err=p.DB.QueryRow(ctx,`SELECT p.name,t.text FROM inspection_transcripts t
 JOIN inspections i ON i.organization_id=t.organization_id AND i.id=t.inspection_id
 JOIN properties p ON p.organization_id=i.organization_id AND p.id=i.property_id
 WHERE t.organization_id=$1 AND t.inspection_id=$2 AND t.media_id=$3`,
 j.OrganizationID,t.InspectionID,t.MediaID).Scan(&propertyName,&transcript)
 if err!=nil{return nil,err}
 // A bounded, one-page PDF summary; transcript remains persisted separately.
 lines:=[]string{"Inspection audio report","Property: "+propertyName,"Inspection: "+t.InspectionID,"Audio: "+t.MediaID,"Transcript excerpt:"}
 for _,line:=range strings.Split(transcript,"\n"){
  for len(line)>88 {lines=append(lines,line[:88]);line=line[88:]}
  lines=append(lines,line)
  if len(lines)>=48{break}
 }
 pdf:=simplePDF(lines)
 _,err=p.DB.Exec(ctx,`INSERT INTO inspection_generated_reports(organization_id,inspection_id,media_id,content)
 VALUES($1,$2,$3,$4) ON CONFLICT (organization_id,inspection_id,media_id)
 DO UPDATE SET content=EXCLUDED.content,updated_at=now()`,j.OrganizationID,t.InspectionID,t.MediaID,pdf)
 if err!=nil{return nil,err}
 return json.Marshal(map[string]any{"bytes":len(pdf),"format":"pdf"})
}
func simplePDF(lines []string) []byte {
 var content strings.Builder
 content.WriteString("BT /F1 10 Tf 40 780 Td 13 TL ")
 for i,line:=range lines {
  if i>0 {content.WriteString("T* ")}
  var clean strings.Builder
  for _,r:=range line {
   if r=='('||r==')'||r=='\\'{clean.WriteRune('\\')}
   if r>=32&&r<=126 {clean.WriteRune(r)} else if unicode.IsSpace(r){clean.WriteByte(' ')}else{clean.WriteByte('?')}
  }
  content.WriteString("("+clean.String()+") Tj ")
 }
 content.WriteString("ET")
 stream:=content.String()
 objects:=[]string{
  "<< /Type /Catalog /Pages 2 0 R >>",
  "<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
  "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 842] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
  "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
  fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream",len(stream),stream),
 }
 var b bytes.Buffer
 b.WriteString("%PDF-1.4\n")
 offsets:=[]int{0}
 for i,obj:=range objects {offsets=append(offsets,b.Len());fmt.Fprintf(&b,"%d 0 obj\n%s\nendobj\n",i+1,obj)}
 xref:=b.Len()
 fmt.Fprintf(&b,"xref\n0 %d\n0000000000 65535 f \n",len(offsets))
 for _,o:=range offsets[1:]{fmt.Fprintf(&b,"%010d 00000 n \n",o)}
 fmt.Fprintf(&b,"trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n",len(offsets),xref)
 return b.Bytes()
}
