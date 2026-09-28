// Package email sends email through Resend's HTTP API.
package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Resend struct {
	APIURL string
	APIKey string
	From   string
	HTTP   *http.Client
}

func NewResend(apiURL, apiKey, from string) *Resend {
	return &Resend{APIURL: apiURL, APIKey: apiKey, From: from, HTTP: &http.Client{Timeout: 15 * time.Second}}
}

// Send sends one email to the given addresses.
func (r *Resend) Send(ctx context.Context, to []string, subject, html string) error {
	body, err := json.Marshal(map[string]any{"from": r.From, "to": to, "subject": subject, "html": html})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.APIURL+"/emails", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.APIKey)
	req.Header.Set("Content-Type", "application/json")
	res, err := r.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("resend: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= http.StatusMultipleChoices {
		detail, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return fmt.Errorf("resend: status %d: %s", res.StatusCode, bytes.TrimSpace(detail))
	}
	return nil
}
