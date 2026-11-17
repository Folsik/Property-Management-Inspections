package api

import (
 "context"
 "fmt"
 "io"
 "log/slog"
 "net/http"
 "net/http/httptest"
 "os"
 "testing"
 "time"

 "github.com/Folsik/Property-Management-Inspections/internal/store"
)
func TestDistributedAuthRateLimitDoesNotTrustSpoofedForwardedIP(t *testing.T){
 if os.Getenv("RUN_DB_INTEGRATION")!="1"{t.Skip("requires PostgreSQL")}
 ctx,cancel:=context.WithTimeout(context.Background(),30*time.Second);defer cancel()
 db,err:=store.Open(ctx,os.Getenv("DATABASE_URL"));if err!=nil{t.Fatal(err)}
 defer db.Close()
 if err=db.Migrate(ctx);err!=nil{t.Fatal(err)}
 s:=New(db.DB,nil,slog.New(slog.NewTextHandler(io.Discard,nil)),"development")
 route:=fmt.Sprintf("integration-rate-%d",time.Now().UnixNano())
 handler:=s.limitAuth(route,2,time.Minute,func(w http.ResponseWriter,r *http.Request){write(w,200,map[string]bool{"ok":true})})
 first:=httptest.NewServer(handler);defer first.Close()
 second:=httptest.NewServer(handler);defer second.Close()
 for i:=0;i<3;i++{
  address:=first.URL
  if i==2{address=second.URL}
  req,err:=http.NewRequestWithContext(ctx,"POST",address+"/",nil);if err!=nil{t.Fatal(err)}
  req.Header.Set("X-Forwarded-For",fmt.Sprintf("10.12.4.%d",i+1))
  response,err:=http.DefaultClient.Do(req);if err!=nil{t.Fatal(err)}
  response.Body.Close()
  wanted:=200;if i==2{wanted=429}
  if response.StatusCode!=wanted{t.Fatalf("request %d: want %d got %d",i,wanted,response.StatusCode)}
 }
}
