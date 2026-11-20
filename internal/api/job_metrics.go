package api

import (
 "net/http"
 "time"
)

// Job metrics are tenant-scoped and authenticated; do not export job payloads.
func (s *Server) jobMetrics(w http.ResponseWriter,r *http.Request){
 if _,ok:=s.role(r,"owner","manager");!ok{fail(w,403,"forbidden");return}
 var pending,running,succeeded,failed int64
 var oldestSeconds float64
 err:=s.db.QueryRow(r.Context(),`
 SELECT count(*) FILTER(WHERE state='pending'),
 count(*) FILTER(WHERE state='running'),
 count(*) FILTER(WHERE state='succeeded'),
 count(*) FILTER(WHERE state='failed'),
 COALESCE(EXTRACT(EPOCH FROM (now()-MIN(created_at) FILTER(WHERE state='pending'))),0)
 FROM jobs WHERE organization_id=$1`,r.PathValue("org")).
 Scan(&pending,&running,&succeeded,&failed,&oldestSeconds)
 if err!=nil{fail(w,503,"unavailable");return}
 w.Header().Set("Cache-Control","private, no-store")
 write(w,200,map[string]any{
 "pending":pending,"running":running,"succeeded":succeeded,"dead_letter":failed,
 "oldest_pending_age_seconds":oldestSeconds,"timestamp":time.Now().UTC(),
 })
}
