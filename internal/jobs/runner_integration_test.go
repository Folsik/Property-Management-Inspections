package jobs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Folsik/Property-Management-Inspections/internal/store"
)

func TestWorkerLeaseRecovery(t *testing.T) {
	if os.Getenv("RUN_DB_INTEGRATION") != "1" {
		t.Skip("requires integration database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := store.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	salt := make([]byte, 6)
	if _, err = rand.Read(salt); err != nil {
		t.Fatal(err)
	}
	suffix := hex.EncodeToString(salt)
	phone := fmt.Sprintf("+447%010d", time.Now().UnixNano()%10000000000)
	var user, org, id string
	err = db.DB.QueryRow(ctx, "INSERT INTO users(phone) VALUES($1) RETURNING id", phone).Scan(&user)
	if err != nil {
		t.Fatal(err)
	}
	defer db.DB.Exec(context.Background(), "DELETE FROM users WHERE id=$1", user)
	err = db.DB.QueryRow(ctx, "INSERT INTO organizations(name,created_by) VALUES($1,$2) RETURNING id", "worker-test-"+suffix, user).Scan(&org)
	if err != nil {
		t.Fatal(err)
	}
	defer db.DB.Exec(context.Background(), "DELETE FROM organizations WHERE id=$1", org)
	err = db.DB.QueryRow(ctx, "INSERT INTO jobs(organization_id,kind,payload,max_attempts) VALUES($1,'extract','{}',2) RETURNING id", org).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	r := &Runner{DB: db.DB, Lease: time.Minute}
	first, err := r.Claim(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if first == nil || first.ID != id {
		t.Fatalf("unexpected claim: %#v", first)
	}
	second, err := r.Claim(ctx)
	if err != nil || second != nil {
		t.Fatalf("duplicate claim: %#v %v", second, err)
	}
	stale := *first
	stale.LeaseToken = "00000000-0000-0000-0000-000000000000"
	if err := r.Finish(ctx, stale, nil, nil); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("stale worker accepted: %v", err)
	}
	if err := r.Finish(ctx, *first, nil, errors.New("temporary fault")); err != nil {
		t.Fatal(err)
	}
	_, err = db.DB.Exec(ctx, "UPDATE jobs SET run_after=now()-interval '1 second' WHERE id=$1", id)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := r.Claim(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if retry == nil || retry.Attempts != 2 {
		t.Fatalf("retry unavailable: %#v", retry)
	}
	if err := r.Finish(ctx, *retry, []byte("{}"), nil); err != nil {
		t.Fatal(err)
	}
	if err := r.Finish(ctx, *first, nil, nil); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("old lease accepted: %v", err)
	}
	var state string
	err = db.DB.QueryRow(ctx, "SELECT state FROM jobs WHERE id=$1", id).Scan(&state)
	if err != nil || state != "succeeded" {
		t.Fatalf("bad final state %s: %v", state, err)
	}
}
func TestDeadLetterAfterExpiredMaxLease(t *testing.T) {
	if os.Getenv("RUN_DB_INTEGRATION") != "1" {
		t.Skip("requires integration database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := store.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	salt := make([]byte, 5)
	if _, err = rand.Read(salt); err != nil {
		t.Fatal(err)
	}
	phone := fmt.Sprintf("+447%010d", time.Now().UnixNano()%10000000000)
	var user, org, id string
	if err = db.DB.QueryRow(ctx, "INSERT INTO users(phone) VALUES($1) RETURNING id", phone).Scan(&user); err != nil {
		t.Fatal(err)
	}
	defer db.DB.Exec(context.Background(), "DELETE FROM users WHERE id=$1", user)
	if err = db.DB.QueryRow(ctx, "INSERT INTO organizations(name,created_by) VALUES($1,$2) RETURNING id", hex.EncodeToString(salt), user).Scan(&org); err != nil {
		t.Fatal(err)
	}
	defer db.DB.Exec(context.Background(), "DELETE FROM organizations WHERE id=$1", org)
	if err = db.DB.QueryRow(ctx, "INSERT INTO jobs(organization_id,kind,max_attempts) VALUES($1,'extract',1) RETURNING id", org).Scan(&id); err != nil {
		t.Fatal(err)
	}
	r := &Runner{DB: db.DB, Lease: time.Minute}
	j, err := r.Claim(ctx)
	if err != nil || j == nil {
		t.Fatalf("claim: %v", err)
	}
	if _, err = db.DB.Exec(ctx, "UPDATE jobs SET locked_until=now()-interval '1 second' WHERE id=$1", id); err != nil {
		t.Fatal(err)
	}
	if err := r.ReapExhausted(ctx); err != nil {
		t.Fatal(err)
	}
	if j2, err := r.Claim(ctx); err != nil || j2 != nil {
		t.Fatalf("reclaimed exhausted job: %#v %v", j2, err)
	}
	var state string
	if err := db.DB.QueryRow(ctx, "SELECT state FROM jobs WHERE id=$1", id).Scan(&state); err != nil || state != "failed" {
		t.Fatalf("expected dead letter: %s %v", state, err)
	}
}
func TestPDFHeader(t *testing.T) {
	data := simplePDF([]string{"Hello world", "Inspection 123"})
	if len(data) < 100 || string(data[:8]) != "%PDF-1.4" {
		t.Fatal("invalid PDF header")
	}
}
