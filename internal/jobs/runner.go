package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Job struct {
	ID             string
	OrganizationID string
	Kind           string
	Payload        json.RawMessage
	Attempts       int
	MaxAttempts    int
	LeaseToken     string
}
type Handler func(context.Context, Job) (json.RawMessage, error)
type Runner struct {
	DB       *pgxpool.Pool
	Logger   *slog.Logger
	Handlers map[string]Handler
	Lease    time.Duration
	Poll     time.Duration
}

var ErrLeaseLost = errors.New("job lease lost")

func Backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 10 {
		attempt = 10
	}
	n := math.Pow(2, float64(attempt-1))
	return time.Duration(n) * 5 * time.Second
}
func (r *Runner) defaults() {
	if r.Lease <= 0 {
		r.Lease = 60 * time.Second
	}
	if r.Poll <= 0 {
		r.Poll = 2 * time.Second
	}
	if r.Logger == nil {
		r.Logger = slog.Default()
	}
}
func (r *Runner) Claim(ctx context.Context) (*Job, error) {
	r.defaults()
	// A single statement guarantees SKIP LOCKED claim across all replicas.
	// Reclaim expired running jobs, never a running job with an active lease.
	var j Job
	err := r.DB.QueryRow(ctx, `
 UPDATE jobs SET state='running',attempts=attempts+1,lease_token=gen_random_uuid(),
 locked_until=now()+($1::double precision * interval '1 second'),updated_at=now()
 WHERE id=(SELECT id FROM jobs WHERE
   (state='pending' AND run_after<=now() AND attempts<max_attempts) OR (state='running' AND locked_until<now() AND attempts<max_attempts)
 ORDER BY run_after,created_at,id FOR UPDATE SKIP LOCKED LIMIT 1)
 RETURNING id,organization_id,kind,payload,attempts,max_attempts,lease_token
 `, r.Lease.Seconds()).Scan(&j.ID, &j.OrganizationID, &j.Kind, &j.Payload, &j.Attempts, &j.MaxAttempts, &j.LeaseToken)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("claim job: %w", err)
	}
	_, err = r.DB.Exec(ctx, `INSERT INTO job_events(job_id,organization_id,event,attempt) VALUES($1,$2,'claimed',$3)`, j.ID, j.OrganizationID, j.Attempts)
	if err != nil {
		r.Logger.Warn("job claim audit failure", "job_id", j.ID, "error", err)
	}
	return &j, nil
}
func (r *Runner) Renew(ctx context.Context, j Job) error {
	tag, err := r.DB.Exec(ctx, `UPDATE jobs SET locked_until=now()+($3::double precision * interval '1 second'),updated_at=now()
 WHERE id=$1 AND lease_token=$2 AND state='running' AND locked_until>now()`, j.ID, j.LeaseToken, r.Lease.Seconds())
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrLeaseLost
	}
	return nil
}
func (r *Runner) Finish(ctx context.Context, j Job, result json.RawMessage, jobErr error) error {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var state string
	var event string
	var message string
	var delay time.Duration
	if jobErr == nil {
		state = "succeeded"
		event = "succeeded"
	} else {
		message = jobErr.Error()
		if len(message) > 1024 {
			message = message[:1024]
		}
		if j.Attempts >= j.MaxAttempts {
			state = "failed"
			event = "dead_lettered"
		} else {
			state = "pending"
			event = "retried"
			delay = Backoff(j.Attempts)
		}
	}
	if len(result) == 0 {
		result = []byte("{}")
	}
	if !json.Valid(result) {
		return errors.New("invalid job result")
	}
	tag, err := tx.Exec(ctx, `
 UPDATE jobs SET state=$3,locked_until=NULL,lease_token=NULL,
 last_error=$4,result=CASE WHEN $3='succeeded' THEN $5::jsonb ELSE result END,
 run_after=CASE WHEN $3='pending' THEN now()+($6::double precision * interval '1 second') ELSE run_after END,
 finished_at=CASE WHEN $3 IN ('succeeded','failed') THEN now() ELSE NULL END,updated_at=now()
 WHERE id=$1 AND lease_token=$2 AND state='running' AND locked_until>now()
 `, j.ID, j.LeaseToken, state, message, string(result), delay.Seconds())
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrLeaseLost
	}
	_, err = tx.Exec(ctx, `INSERT INTO job_events(job_id,organization_id,event,attempt,detail) VALUES($1,$2,$3,$4,$5)`, j.ID, j.OrganizationID, event, j.Attempts, message)
	if err != nil {
		return err
	}
	if jobErr == nil {
		next := ""
		if j.Kind == "transcribe" {
			next = "extract"
		}
		if j.Kind == "extract" {
			next = "generate_pdf"
		}
		if next != "" {
			_, err = tx.Exec(ctx, `INSERT INTO jobs(organization_id,kind,payload,idempotency_key)
    VALUES($1,$2,$3,$4)
    ON CONFLICT (organization_id,kind,idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING`,
				j.OrganizationID, next, string(j.Payload), j.ID)
			if err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}

// ReapExhausted transitions crashed tasks with no retry budget to dead letters.
func (r *Runner) ReapExhausted(ctx context.Context) error {
	_, err := r.DB.Exec(ctx, `
 WITH exhausted AS (
 UPDATE jobs SET state='failed',lease_token=NULL,locked_until=NULL,
 last_error='lease expired after maximum attempts',finished_at=now(),updated_at=now()
 WHERE state='running' AND locked_until<now() AND attempts>=max_attempts
 RETURNING id,organization_id,attempts
 )
 INSERT INTO job_events(job_id,organization_id,event,attempt,detail)
 SELECT id,organization_id,'dead_lettered',attempts,'lease expired after maximum attempts' FROM exhausted
 `)
	return err
}
func (r *Runner) ProcessOne(ctx context.Context) (bool, error) {
	j, err := r.Claim(ctx)
	if err != nil {
		return false, err
	}
	if j == nil {
		return false, nil
	}
	handler := r.Handlers[j.Kind]
	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(r.Lease / 3)
		defer ticker.Stop()
		defer close(done)
		for {
			select {
			case <-workCtx.Done():
				return
			case <-ticker.C:
				renewalCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
				err := r.Renew(renewalCtx, *j)
				stop()
				if err != nil {
					r.Logger.Error("job lease renewal failed", "job_id", j.ID, "error", err)
					cancel()
					return
				}
			}
		}
	}()
	var output json.RawMessage
	var handlerErr error
	if handler == nil {
		handlerErr = fmt.Errorf("no handler registered for job kind %s", j.Kind)
	} else {
		output, handlerErr = handler(workCtx, *j)
	}
	cancel()
	<-done
	finalizeCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
	defer stop()
	if err := r.Finish(finalizeCtx, *j, output, handlerErr); err != nil {
		return true, err
	}
	if handlerErr != nil {
		r.Logger.Warn("job attempt failed", "job_id", j.ID, "kind", j.Kind, "attempt", j.Attempts, "error", handlerErr)
	}
	return true, nil
}
func (r *Runner) Run(ctx context.Context) error {
	r.defaults()
	for {
		if ctx.Err() != nil {
			return nil
		}
		// A crashed last-attempt worker cannot renew its lease; promote it to
		// dead letter before claiming additional work.
		if err := r.ReapExhausted(ctx); err != nil {
			r.Logger.Error("job dead-letter reaper failed", "error", err)
		}
		processed, err := r.ProcessOne(ctx)
		if err != nil {
			r.Logger.Error("worker operation failed", "error", err)
		}
		if !processed || err != nil {
			timer := time.NewTimer(r.Poll)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil
			case <-timer.C:
			}
		}
	}
}
