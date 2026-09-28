// Package fakeresend imitates the part of Resend's API that postik uses:
// sending one email.
package fakeresend

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
)

const APIKey = "re_fake"

// Email is a message Resend was asked to send.
type Email struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
}

type Server struct {
	mu     sync.Mutex
	emails []Email
}

func New() *Server {
	return &Server{}
}

// Emails lists, in order, the emails received.
func (s *Server) Emails() []Email {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Email(nil), s.emails...)
}

// To lists the emails sent to an address.
func (s *Server) To(address string) []Email {
	var out []Email
	for _, e := range s.Emails() {
		for _, to := range e.To {
			if to == address {
				out = append(out, e)
			}
		}
	}
	return out
}

// Handler serves POST /emails and GET /_control/emails.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /emails", s.send)
	mux.HandleFunc("GET /_control/emails", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		emails := s.Emails()
		if emails == nil {
			emails = []Email{}
		}
		_ = json.NewEncoder(w).Encode(emails)
	})
	return mux
}

func (s *Server) send(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Header.Get("Authorization") != "Bearer "+APIKey {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{"statusCode": 401, "name": "validation_error", "message": "API key is invalid"})
		return
	}
	var email Email
	if err := json.NewDecoder(r.Body).Decode(&email); err != nil || email.From == "" || len(email.To) == 0 || email.Subject == "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{"statusCode": 422, "name": "validation_error", "message": "from, to and subject are required"})
		return
	}
	s.mu.Lock()
	s.emails = append(s.emails, email)
	id := strconv.Itoa(len(s.emails))
	s.mu.Unlock()
	_ = json.NewEncoder(w).Encode(map[string]string{"id": "fake-" + id})
}
