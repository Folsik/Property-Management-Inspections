package api

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgconn"
)

// Security audit payloads intentionally exclude passwords, tokens, phone,
// email, user agents and raw request paths.
func (s *Server) auditSecurityEvent(r *http.Request, actor, org, action, resourceType, resourceID string) {
	var actorID, organizationID, targetID any
	if actor != "" {
		actorID = actor
	}
	if org != "" {
		organizationID = org
	}
	if resourceID != "" {
		targetID = resourceID
	}
	_, err := s.db.Exec(r.Context(), `INSERT INTO audit_log
 (organization_id,actor_id,action,resource_type,resource_id,metadata)
 VALUES($1,$2,$3,$4,$5,jsonb_build_object('request_id',$6::text))`,
		organizationID, actorID, action, resourceType, targetID, requestIDFromContext(r.Context()))
	if err != nil {
		s.logDatabaseError(r, "security audit insert", err)
	}
}

func (s *Server) logDatabaseError(r *http.Request, operation string, err error) {
	if err == nil {
		return
	}
	if pgErr, ok := err.(*pgconn.PgError); ok {
		s.logger.Error("database operation failed", "request_id", requestIDFromContext(r.Context()),
			"operation", operation, "sqlstate", pgErr.Code, "constraint", pgErr.ConstraintName)
		return
	}
	s.logger.Error("database operation failed", "request_id", requestIDFromContext(r.Context()),
		"operation", operation, "error", err)
}
