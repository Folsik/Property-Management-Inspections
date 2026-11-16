package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"time"
)

func normalizedEmail(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) > 254 || len(value) < 3 || strings.ContainsAny(value, " \t\r\n") {
		return ""
	}
	address, err := mail.ParseAddress(value)
	if err != nil || address.Address != value || strings.Count(value, "@") != 1 {
		return ""
	}
	return value
}

func sendTransactionalEmail(ctx context.Context, to, subject, html string) error {
	key := os.Getenv("RESEND_API_KEY")
	from := os.Getenv("EMAIL_FROM")
	if key == "" || from == "" {
		return fmt.Errorf("email provider not configured")
	}
	body, err := json.Marshal(map[string]any{"from": from, "to": []string{to}, "subject": subject, "html": html})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("email provider returned HTTP %d", response.StatusCode)
	}
	return nil
}
