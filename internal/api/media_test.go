package api
import("net/url";"strings";"testing";"time")
func TestPresignPUT(t *testing.T){
 u,err:=presignPUT("https://example.r2.cloudflarestorage.com","testbucket","auto","access","secret","organizations/org/abc","image/jpeg",time.Date(2026,11,16,10,0,0,0,time.UTC))
 if err!=nil{t.Fatal(err)}
 p,e:=url.Parse(u);if e!=nil{t.Fatal(e)}
 if !strings.HasPrefix(p.Path,"/testbucket/organizations/")||p.Query().Get("X-Amz-Signature")==""||p.Query().Get("X-Amz-Expires")!="300"{t.Fatalf("bad signed URL: %s",u)}
}
func TestPresignRejectsHTTP(t *testing.T){if _,err:=presignPUT("http://host","bucket","auto","a","s","key","image/jpeg",time.Now());err==nil{t.Fatal("accepted HTTP endpoint")}}
