package api

import (
 "encoding/base64"
 "encoding/json"
 "errors"
 "net/http"
 "time"
)
type seekCursor struct {
 Org string `json:"o"`
 Kind string `json:"k"`
 Created time.Time `json:"t"`
 ID string `json:"i"`
}
func decodeSeekCursor(raw,org,kind string)(time.Time,string,error){
 if raw==""{return time.Now().UTC().Add(24*time.Hour),"ffffffff-ffff-ffff-ffff-ffffffffffff",nil}
 if len(raw)>512{return time.Time{},"",errors.New("invalid page cursor")}
 data,err:=base64.RawURLEncoding.DecodeString(raw)
 if err!=nil{return time.Time{},"",errors.New("invalid page cursor")}
 var cur seekCursor
 if json.Unmarshal(data,&cur)!=nil||cur.Org!=org||cur.Kind!=kind||cur.Created.IsZero()||!validID(cur.ID){
  return time.Time{},"",errors.New("invalid page cursor")
 }
 return cur.Created,cur.ID,nil
}
func encodeSeekCursor(org,kind string,created time.Time,id string)string{
 bytes,_:=json.Marshal(seekCursor{org,kind,created,id})
 return base64.RawURLEncoding.EncodeToString(bytes)
}
func pageOptions(r *http.Request,kind string,defaultLimit,max int)(int,time.Time,string,error){
 size,err:=queryPageLimit(r,defaultLimit,max)
 if err!=nil{return 0,time.Time{},"",err}
 created,id,err:=decodeSeekCursor(r.URL.Query().Get("cursor"),r.PathValue("org"),kind)
 return size,created,id,err
}
