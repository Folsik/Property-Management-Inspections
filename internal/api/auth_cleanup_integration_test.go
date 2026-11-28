package api

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Folsik/Property-Management-Inspections/internal/store"
)

func TestCleanupExpiredAuthRetention(t *testing.T) {
	if os.Getenv("RUN_DB_INTEGRATION") != "1" {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := store.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	s := &Server{db: db.DB}
	prefix := fmt.Sprintf("cleanup-%d-", time.Now().UnixNano())
	for _, tc := range []struct {
		table, key, timestamp, oldAge, currentAge string
	}{
		{"revoked_tokens", "jti", "expires_at", "2 hours", "-1 hour"},
		{"auth_rate_limits", "scope_hash", "window_started_at", "2 hours", "10 minutes"},
	} {
		for _, row := range []struct{ suffix, age string }{{"old", tc.oldAge}, {"current", tc.currentAge}} {
			query := fmt.Sprintf("INSERT INTO %s(%s,%s) VALUES($1,now()-$2::interval)", tc.table, tc.key, tc.timestamp)
			if _, err := db.DB.Exec(ctx, query, prefix+row.suffix, row.age); err != nil {
				t.Fatal(err)
			}
		}
	}
	phone := fmt.Sprintf("+1777%08d", time.Now().UnixNano()%100000000)
	for _, age := range []string{"2 hours", "10 minutes", "-5 minutes"} {
		if _, err := db.DB.Exec(ctx, "INSERT INTO otp_challenges(phone,code_hash,expires_at) VALUES($1,'test',now()-$2::interval)", phone, age); err != nil {
			t.Fatal(err)
		}
	}
	// Preserve replay hashes for theft detection, but erase encrypted
	// replacement credentials as soon as the grace window closes.
	var user, session string
	refreshPhone := fmt.Sprintf("+1888%08d", time.Now().UnixNano()%100000000)
	if err := db.DB.QueryRow(ctx, "INSERT INTO users(phone) VALUES($1) RETURNING id", refreshPhone).Scan(&user); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.QueryRow(ctx, `INSERT INTO auth_sessions(user_id,current_token_hash,auth_epoch,expires_at)
 VALUES($1,$2,0,now()+interval '1 day') RETURNING id`, user, prefix+"current").Scan(&session); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct{ suffix, age, grace string }{
		{"expired", "2 hours", "2 hours"},
		{"spent", "-1 hour", "2 hours"},
		{"retry", "-1 hour", "-1 minute"},
	} {
		if _, err := db.DB.Exec(ctx, `INSERT INTO used_refresh_tokens
 (token_hash,session_id,expires_at,replay_response,replay_until)
 VALUES($1,$2,now()-$3::interval,$4,now()-$5::interval)`,
			prefix+row.suffix, session, row.age, []byte("encrypted test credential"), row.grace); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		if _, err := s.CleanupExpiredAuth(ctx); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct{ table, key string }{{"revoked_tokens", "jti"}, {"auth_rate_limits", "scope_hash"}} {
		var count int
		query := fmt.Sprintf("SELECT count(*) FROM %s WHERE %s=$1", tc.table, tc.key)
		if err := db.DB.QueryRow(ctx, query, prefix+"old").Scan(&count); err != nil || count != 0 {
			t.Fatalf("%s old: count=%d err=%v", tc.table, count, err)
		}
		if err := db.DB.QueryRow(ctx, query, prefix+"current").Scan(&count); err != nil || count != 1 {
			t.Fatalf("%s current: count=%d err=%v", tc.table, count, err)
		}
	}
	var expired int
	if err := db.DB.QueryRow(ctx, "SELECT count(*) FROM used_refresh_tokens WHERE token_hash=$1", prefix+"expired").Scan(&expired); err != nil || expired != 0 {
		t.Fatalf("expired refresh cleanup: count=%d err=%v", expired, err)
	}
	for _, row := range []struct {
		suffix string
		cached bool
	}{{"spent", false}, {"retry", true}} {
		var cached bool
		err := db.DB.QueryRow(ctx, "SELECT replay_response IS NOT NULL FROM used_refresh_tokens WHERE token_hash=$1", prefix+row.suffix).Scan(&cached)
		if err != nil || cached != row.cached {
			t.Fatalf("refresh retry retention %s: cached=%v err=%v", row.suffix, cached, err)
		}
	}
	var count int
	if err := db.DB.QueryRow(ctx, "SELECT count(*) FROM otp_challenges WHERE phone=$1", phone).Scan(&count); err != nil || count != 2 {
		t.Fatalf("OTP retention: count=%d err=%v", count, err)
	}
}
