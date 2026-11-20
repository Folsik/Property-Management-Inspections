package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

type syncCursor struct {
	Org string `json:"o"`
	Seq int64  `json:"s"`
}

func encodeSyncCursor(org string, seq int64) string {
	data, _ := json.Marshal(syncCursor{Org: org, Seq: seq})
	return base64.RawURLEncoding.EncodeToString(data)
}
func decodeSyncCursor(value, org string) (int64, error) {
	if value == "" {
		return 0, nil
	}
	if len(value) > 256 {
		return 0, errors.New("invalid cursor")
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return 0, errors.New("invalid cursor")
	}
	var cursor syncCursor
	if json.Unmarshal(raw, &cursor) != nil || cursor.Org != org || cursor.Seq < 0 {
		return 0, errors.New("invalid cursor")
	}
	return cursor.Seq, nil
}
func queryPageLimit(r *http.Request, defaultLimit, max int) (int, error) {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return defaultLimit, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > max {
		return 0, errors.New("limit must be within 1.." + strconv.Itoa(max))
	}
	return n, nil
}
func (s *Server) pullSync(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.role(r, "owner", "manager", "inspector"); !ok {
		fail(w, 403, "forbidden")
		return
	}
	org := r.PathValue("org")
	seq, err := decodeSyncCursor(r.URL.Query().Get("cursor"), org)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	limit, err := queryPageLimit(r, 100, 500)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	rows, err := s.db.Query(r.Context(), `SELECT seq,entity_type,entity_id,version,deleted,payload,updated_at
 FROM sync_changes WHERE organization_id=$1 AND seq>$2 ORDER BY seq LIMIT $3`, org, seq, limit+1)
	if err != nil {
		fail(w, 503, "unavailable")
		return
	}
	defer rows.Close()
	changes := make([]map[string]any, 0, limit+1)
	last := seq
	for rows.Next() {
		var number, version int64
		var kind, id string
		var deleted bool
		var payload []byte
		var updated any
		if err = rows.Scan(&number, &kind, &id, &version, &deleted, &payload, &updated); err != nil {
			fail(w, 503, "unavailable")
			return
		}
		changes = append(changes, map[string]any{"seq": number, "entity": kind, "id": id, "version": version,
			"deleted": deleted, "data": json.RawMessage(payload), "updated_at": updated})
		if len(changes) <= limit {
			last = number
		}
	}
	if rows.Err() != nil {
		fail(w, 503, "unavailable")
		return
	}
	more := len(changes) > limit
	if more {
		changes = changes[:limit]
	}
	w.Header().Set("Cache-Control", "no-store")
	write(w, 200, map[string]any{"changes": changes, "next_cursor": encodeSyncCursor(org, last), "has_more": more})
}

type syncMutation struct {
	MutationID   string `json:"mutation_id"`
	Entity       string `json:"entity"`
	Action       string `json:"action"`
	InspectionID string `json:"inspection_id"`
	ID           string `json:"id"`
	ClientID     string `json:"client_id"`
	Version      *int64 `json:"version"`
	Title        string `json:"title"`
	Notes        string `json:"notes"`
	Severity     string `json:"severity"`
	Status       string `json:"status"`
}

func (s *Server) pushSync(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.role(r, "owner", "manager", "inspector"); !ok {
		fail(w, 403, "forbidden")
		return
	}
	var in struct {
		Mutations []syncMutation `json:"mutations"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(in.Mutations) < 1 || len(in.Mutations) > 50 {
		fail(w, 400, "provide 1 to 50 mutations")
		return
	}
	org, user := r.PathValue("org"), who(r).UserID
	results := make([]map[string]any, 0, len(in.Mutations))
	for _, m := range in.Mutations {
		if !validID(m.MutationID) || m.Entity != "inspection_item" ||
			(m.Action != "create" && m.Action != "update" && m.Action != "delete") ||
			!validID(m.InspectionID) ||
			(m.Action == "create" && !validID(m.ClientID)) ||
			(m.Action != "create" && (!validID(m.ID) || m.Version == nil || *m.Version < 1)) {
			fail(w, 400, "invalid sync mutation")
			return
		}
		if m.Action == "create" {
			m.Title = strings.TrimSpace(m.Title)
			if len(m.Title) < 1 || len(m.Title) > 500 || len(m.Notes) > 10000 {
				fail(w, 400, "invalid item")
				return
			}
		}
		if m.Severity != "" && m.Severity != "normal" && m.Severity != "low" && m.Severity != "high" && m.Severity != "critical" {
			fail(w, 400, "invalid severity")
			return
		}
		if m.Status != "" && m.Status != "open" && m.Status != "in_progress" && m.Status != "resolved" {
			fail(w, 400, "invalid status")
			return
		}
		// Canonical Go JSON encoding yields stable hashes for exactly the same decoded request.
		canonical, _ := json.Marshal(m)
		digest := sha256.Sum256(canonical)
		tx, err := s.db.Begin(r.Context())
		if err != nil {
			fail(w, 503, "unavailable")
			return
		}
		var existing []byte
		var output []byte
		var storedStatus *int
		err = tx.QueryRow(r.Context(), `SELECT request_hash,response_body,response_code FROM idempotency_keys
   WHERE organization_id=$1 AND actor_id=$2 AND key=$3 FOR UPDATE`, org, user, m.MutationID).
			Scan(&existing, &output, &storedStatus)
		if err == nil {
			tx.Rollback(r.Context())
			if !bytes.Equal(existing, digest[:]) {
				fail(w, 409, "idempotency key reused with different content")
				return
			}
			var result map[string]any
			if json.Unmarshal(output, &result) != nil {
				fail(w, 503, "invalid replay result")
				return
			}
			results = append(results, result)
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			tx.Rollback(r.Context())
			fail(w, 503, "unavailable")
			return
		}
		// Reserve unique key under the transaction. A concurrent duplicate waits,
		// then can replay the committed value without reapplying its mutation.
		tag, err := tx.Exec(r.Context(), `INSERT INTO idempotency_keys(organization_id,actor_id,key,request_hash)
    VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, org, user, m.MutationID, digest[:])
		if err != nil {
			tx.Rollback(r.Context())
			fail(w, 503, "unavailable")
			return
		}
		if tag.RowsAffected() == 0 {
			tx.Rollback(r.Context())
			fail(w, 409, "mutation already being processed; retry")
			return
		}
		result, err := applySyncItem(r, tx, org, m)
		if err != nil {
			tx.Rollback(r.Context())
			fail(w, 503, "sync unavailable")
			return
		}
		raw, _ := json.Marshal(result)
		_, err = tx.Exec(r.Context(), `UPDATE idempotency_keys SET response_code=$4,response_body=$5
   WHERE organization_id=$1 AND actor_id=$2 AND key=$3`, org, user, m.MutationID, result["status"], raw)
		if err != nil {
			tx.Rollback(r.Context())
			fail(w, 503, "sync unavailable")
			return
		}
		if err = tx.Commit(r.Context()); err != nil {
			fail(w, 503, "sync unavailable")
			return
		}
		results = append(results, result)
	}
	w.Header().Set("Cache-Control", "no-store")
	write(w, 200, map[string]any{"results": results})
}
func applySyncItem(r *http.Request, tx pgx.Tx, org string, m syncMutation) (map[string]any, error) {
	ctx := r.Context()
	result := map[string]any{"mutation_id": m.MutationID, "entity": m.Entity, "action": m.Action}
	if m.Action == "create" {
		severity := m.Severity
		if severity == "" {
			severity = "normal"
		}
		var id string
		var version int64
		err := tx.QueryRow(ctx, `INSERT INTO inspection_items(organization_id,inspection_id,client_id,title,notes,severity)
   SELECT organization_id,id,$3,$4,$5,$6 FROM inspections
   WHERE organization_id=$1 AND id=$2 AND deleted_at IS NULL AND status IN ('draft','in_progress')
   ON CONFLICT (inspection_id,client_id) DO NOTHING RETURNING id,version`,
			org, m.InspectionID, m.ClientID, m.Title, m.Notes, severity).Scan(&id, &version)
		if errors.Is(err, pgx.ErrNoRows) {
			var priorTitle, priorNotes, priorSeverity string
			err = tx.QueryRow(ctx, `SELECT id,version,title,notes,severity FROM inspection_items
    WHERE organization_id=$1 AND inspection_id=$2 AND client_id=$3 AND deleted_at IS NULL`,
				org, m.InspectionID, m.ClientID).Scan(&id, &version, &priorTitle, &priorNotes, &priorSeverity)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					result["status"] = 409
					result["error"] = "inspection unavailable"
					return result, nil
				}
				return nil, err
			}
			if priorTitle != m.Title || priorNotes != m.Notes || priorSeverity != severity {
				result["status"] = 409
				result["error"] = "client_id already belongs to different item"
				return result, nil
			}
		} else if err != nil {
			return nil, err
		}
		result["status"] = 201
		result["id"] = id
		result["version"] = version
		return result, nil
	}
	var id string
	var version int64
	var current []byte
	if m.Action == "update" {
		if m.Status == "" && m.Title == "" && m.Notes == "" && m.Severity == "" {
			result["status"] = 400
			result["error"] = "nothing to update"
			return result, nil
		}
		err := tx.QueryRow(ctx, `UPDATE inspection_items SET
   title=CASE WHEN $6<>'' THEN $6 ELSE title END,
   notes=CASE WHEN $7<>'' THEN $7 ELSE notes END,
   severity=CASE WHEN $8<>'' THEN $8 ELSE severity END,
   status=CASE WHEN $9<>'' THEN $9 ELSE status END
   WHERE organization_id=$1 AND inspection_id=$2 AND id=$3 AND version=$4 AND deleted_at IS NULL
   AND EXISTS(SELECT 1 FROM inspections WHERE organization_id=$1 AND id=$2 AND status IN ('draft','in_progress') AND deleted_at IS NULL)
   RETURNING id,version`, org, m.InspectionID, m.ID, *m.Version, true, m.Title, m.Notes, m.Severity, m.Status).Scan(&id, &version)
		if err == nil {
			result["status"] = 200
			result["id"] = id
			result["version"] = version
			return result, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	} else {
		err := tx.QueryRow(ctx, `UPDATE inspection_items SET deleted_at=now()
   WHERE organization_id=$1 AND inspection_id=$2 AND id=$3 AND version=$4 AND deleted_at IS NULL
   RETURNING id,version`, org, m.InspectionID, m.ID, *m.Version).Scan(&id, &version)
		if err == nil {
			result["status"] = 200
			result["id"] = id
			result["version"] = version
			result["deleted"] = true
			return result, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}
	err := tx.QueryRow(ctx, `SELECT to_jsonb(i) FROM inspection_items i WHERE organization_id=$1 AND inspection_id=$2 AND id=$3`,
		org, m.InspectionID, m.ID).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		result["status"] = 404
		result["error"] = "item not found"
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	result["status"] = 409
	result["error"] = "stale version; resolve conflict"
	result["current"] = json.RawMessage(current)
	return result, nil
}

var _ = fmt.Sprintf
