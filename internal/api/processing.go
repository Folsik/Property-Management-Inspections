package api

import (
	"net/http"
)

func (s *Server) enqueueTranscription(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.role(r, "owner", "manager", "inspector"); !ok {
		fail(w, 403, "forbidden")
		return
	}
	org, inspection := r.PathValue("org"), r.PathValue("inspection")
	if !validID(inspection) {
		fail(w, 400, "invalid inspection")
		return
	}
	var in struct {
		MediaID        string `json:"media_id"`
		IdempotencyKey string `json:"idempotency_key"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !validID(in.MediaID) || !validID(in.IdempotencyKey) {
		fail(w, 400, "invalid media or idempotency key")
		return
	}
	var valid bool
	err := s.db.QueryRow(r.Context(), `SELECT EXISTS(
 SELECT 1 FROM media_attachments WHERE organization_id=$1 AND inspection_id=$2 AND id=$3
 AND state='uploaded' AND deleted_at IS NULL AND mime_type IN ('audio/mp4','audio/mpeg')
 )`, org, inspection, in.MediaID).Scan(&valid)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	if !valid {
		fail(w, 404, "verified audio not found")
		return
	}
	var id string
	err = s.db.QueryRow(r.Context(), `
 INSERT INTO jobs(organization_id,kind,payload,idempotency_key)
 VALUES($1,'transcribe',jsonb_build_object('inspection_id',$2::text,'media_id',$3::text),$4)
 ON CONFLICT (organization_id,kind,idempotency_key) WHERE idempotency_key IS NOT NULL
 DO UPDATE SET idempotency_key=EXCLUDED.idempotency_key
 RETURNING id`, org, inspection, in.MediaID, in.IdempotencyKey).Scan(&id)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	// Existing jobs with reused keys must have identical content.
	var matches bool
	err = s.db.QueryRow(r.Context(), `SELECT payload->>'inspection_id'=$2 AND payload->>'media_id'=$3
 FROM jobs WHERE id=$1 AND organization_id=$4`, id, inspection, in.MediaID, org).Scan(&matches)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	if !matches {
		fail(w, 409, "idempotency key reused with different content")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	write(w, 202, map[string]string{"id": id, "status": "queued"})
}
func (s *Server) downloadGeneratedReport(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.role(r, "owner", "manager", "inspector"); !ok {
		fail(w, 403, "forbidden")
		return
	}
	inspection, media := r.PathValue("inspection"), r.PathValue("media")
	if !validID(inspection) || !validID(media) {
		fail(w, 400, "invalid resource")
		return
	}
	var content []byte
	err := s.db.QueryRow(r.Context(), `SELECT content FROM inspection_generated_reports WHERE organization_id=$1 AND inspection_id=$2 AND media_id=$3`, r.PathValue("org"), inspection, media).Scan(&content)
	if err != nil {
		fail(w, 404, "report not available")
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=inspection-report.pdf")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (s *Server) requeueFailedJob(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.role(r, "owner", "manager"); !ok {
		fail(w, 403, "forbidden")
		return
	}
	id := r.PathValue("job")
	if !validID(id) {
		fail(w, 400, "invalid job")
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	var attempts int
	err = tx.QueryRow(r.Context(), `UPDATE jobs SET state='pending',attempts=0,run_after=now(),locked_until=NULL,
 lease_token=NULL,finished_at=NULL,last_error=NULL,updated_at=now()
 WHERE id=$1 AND organization_id=$2 AND state='failed' RETURNING attempts`,
		id, r.PathValue("org")).Scan(&attempts)
	if err != nil {
		fail(w, 404, "failed job not found")
		return
	}
	_, err = tx.Exec(r.Context(), `INSERT INTO job_events(job_id,organization_id,event,attempt)
 VALUES($1,$2,'requeued',$3)`, id, r.PathValue("org"), attempts)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	_, err = tx.Exec(r.Context(), `INSERT INTO audit_log(organization_id,actor_id,action,resource_type,resource_id)
 VALUES($1,$2,'job.requeued','job',$3)`, r.PathValue("org"), who(r).UserID, id)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		fail(w, 503, "unavailable")
		return
	}
	write(w, 202, map[string]string{"id": id, "status": "pending"})
}
