package api

import (
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type contactDTO struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Category   string    `json:"category"`
	Email      string    `json:"email"`
	Phone      string    `json:"phone"`
	Notes      string    `json:"notes"`
	PropertyID *string   `json:"property_id"`
	CreatedAt  time.Time `json:"created_at"`
}

func (s *Server) listContacts(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.role(r, "owner", "manager"); !ok {
		fail(w, 403, "forbidden")
		return
	}
	rows, err := s.db.Query(r.Context(), `SELECT id,name,category,email,phone,notes,property_id,created_at
		FROM contacts WHERE organization_id=$1 ORDER BY created_at DESC LIMIT 200`, r.PathValue("org"))
	if err != nil {
		fail(w, 503, "database unavailable")
		return
	}
	defer rows.Close()
	contacts := make([]contactDTO, 0)
	for rows.Next() {
		var c contactDTO
		if err := rows.Scan(&c.ID, &c.Name, &c.Category, &c.Email, &c.Phone, &c.Notes, &c.PropertyID, &c.CreatedAt); err != nil {
			fail(w, 503, "database unavailable")
			return
		}
		contacts = append(contacts, c)
	}
	if rows.Err() != nil {
		fail(w, 503, "database unavailable")
		return
	}
	write(w, 200, map[string]any{"items": contacts})
}

func (s *Server) createContact(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.role(r, "owner", "manager"); !ok {
		fail(w, 403, "forbidden")
		return
	}
	var in struct {
		Name       string  `json:"name"`
		Category   string  `json:"category"`
		Email      string  `json:"email"`
		Phone      string  `json:"phone"`
		Notes      string  `json:"notes"`
		PropertyID *string `json:"property_id"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.Phone = strings.TrimSpace(in.Phone)
	if len(in.Name) < 1 || len(in.Name) > 160 || len(in.Notes) > 2000 {
		fail(w, 400, "invalid contact")
		return
	}
	switch in.Category {
	case "owner", "tenant", "vendor", "other":
	default:
		fail(w, 400, "invalid contact category")
		return
	}
	if in.Email != "" {
		addr, err := mail.ParseAddress(in.Email)
		if err != nil || addr.Address != in.Email || len(in.Email) > 254 {
			fail(w, 400, "invalid email")
			return
		}
	}
	if in.Phone != "" && !phonePattern.MatchString(in.Phone) {
		fail(w, 400, "invalid phone")
		return
	}
	if in.PropertyID != nil && !validID(*in.PropertyID) {
		fail(w, 400, "invalid property")
		return
	}
	var c contactDTO
	err := s.db.QueryRow(r.Context(), `INSERT INTO contacts
		(organization_id,property_id,created_by,name,category,email,phone,notes)
		SELECT $1,p.id,$2,$3,$4,$5,$6,$7
		FROM (SELECT $8::uuid as requested_property_id) request
		LEFT JOIN properties p ON p.organization_id=$1 AND p.id=request.requested_property_id
		WHERE request.requested_property_id IS NULL OR p.id IS NOT NULL
		RETURNING id,name,category,email,phone,notes,property_id,created_at`,
		r.PathValue("org"), who(r).UserID, in.Name, in.Category, in.Email, in.Phone, in.Notes, in.PropertyID).
		Scan(&c.ID, &c.Name, &c.Category, &c.Email, &c.Phone, &c.Notes, &c.PropertyID, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "property not found")
		return
	}
	if err != nil {
		fail(w, 503, "database unavailable")
		return
	}
	write(w, 201, c)
}
