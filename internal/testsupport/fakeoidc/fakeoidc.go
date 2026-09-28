// Package fakeoidc is an OpenID Connect provider for tests and local
// development: discovery, JWKS, authorization code with PKCE and RS256 tokens.
package fakeoidc

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"math/big"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const (
	ClientID     = "postik"
	ClientSecret = "postik-secret"
	keyID        = "fake-key"
)

// Identity is who signs in. Empty fields are left out of the ID token.
type Identity struct {
	Subject           string `json:"sub"`
	Email             string `json:"email,omitempty"`
	Name              string `json:"name,omitempty"`
	PreferredUsername string `json:"preferred_username,omitempty"`
}

type authorization struct {
	identity      Identity
	nonce         string
	codeChallenge string
	redirectURI   string
}

type Provider struct {
	Issuer string

	key *rsa.PrivateKey
	now func() time.Time

	mu    sync.Mutex
	next  *Identity
	deny  bool
	codes map[string]authorization
}

func New(issuer string, now func() time.Time) (*Provider, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	return &Provider{Issuer: issuer, key: key, now: now, codes: map[string]authorization{}}, nil
}

// SignInNext makes the next authorization approve at once as identity,
// without showing the form.
func (p *Provider) SignInNext(identity Identity) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.next = &identity
	p.deny = false
}

// DenyNext makes the next authorization answer error=access_denied.
func (p *Provider) DenyNext() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.next = nil
	p.deny = true
}

func (p *Provider) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", p.discovery)
	mux.HandleFunc("GET /jwks", p.jwks)
	mux.HandleFunc("GET /authorize", p.authorize)
	mux.HandleFunc("POST /authorize", p.submitForm)
	mux.HandleFunc("POST /token", p.token)
	mux.HandleFunc("POST /_control/next", func(w http.ResponseWriter, r *http.Request) {
		var identity Identity
		if err := json.NewDecoder(r.Body).Decode(&identity); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		p.SignInNext(identity)
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

func (p *Provider) discovery(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"issuer":                                p.Issuer,
		"authorization_endpoint":                p.Issuer + "/authorize",
		"token_endpoint":                        p.Issuer + "/token",
		"jwks_uri":                              p.Issuer + "/jwks",
		"response_types_supported":              []string{"code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"code_challenge_methods_supported":      []string{"S256"},
	})
}

func (p *Provider) jwks(w http.ResponseWriter, _ *http.Request) {
	pub := p.key.PublicKey
	writeJSON(w, map[string]any{"keys": []map[string]string{{
		"kty": "RSA",
		"kid": keyID,
		"use": "sig",
		"alg": "RS256",
		"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}}})
}

var form = template.Must(template.New("form").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>Fake OIDC</title>
<style>body{font-family:sans-serif;background:#0e0e0e;color:#fff;display:flex;justify-content:center;padding-top:80px}
form{display:flex;flex-direction:column;gap:12px;width:320px}input{padding:8px}button{padding:10px;background:#612bd3;color:#fff;border:0}</style>
</head><body><form method="post" action="/authorize">
<h1>Fake OIDC</h1>
<label>Subject <input name="sub" value="ruben" required></label>
<label>Email <input name="email" value="ruben@example.com"></label>
<label>Name <input name="name" value="Rubén"></label>
{{range $k, $v := .}}<input type="hidden" name="{{$k}}" value="{{$v}}">{{end}}
<button type="submit">Sign in</button>
</form></body></html>`))

func (p *Provider) authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	params := map[string]string{
		"redirect_uri":   q.Get("redirect_uri"),
		"state":          q.Get("state"),
		"nonce":          q.Get("nonce"),
		"code_challenge": q.Get("code_challenge"),
	}
	p.mu.Lock()
	next, deny := p.next, p.deny
	p.next, p.deny = nil, false
	p.mu.Unlock()

	switch {
	case deny:
		p.redirect(w, r, params["redirect_uri"], url.Values{"error": {"access_denied"}, "state": {params["state"]}})
	case next != nil:
		p.approve(w, r, *next, params)
	default:
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = form.Execute(w, params)
	}
}

func (p *Provider) submitForm(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	identity := Identity{Subject: r.PostForm.Get("sub"), Email: r.PostForm.Get("email"), Name: r.PostForm.Get("name")}
	p.approve(w, r, identity, map[string]string{
		"redirect_uri":   r.PostForm.Get("redirect_uri"),
		"state":          r.PostForm.Get("state"),
		"nonce":          r.PostForm.Get("nonce"),
		"code_challenge": r.PostForm.Get("code_challenge"),
	})
}

func (p *Provider) approve(w http.ResponseWriter, r *http.Request, identity Identity, params map[string]string) {
	code := randomString()
	p.mu.Lock()
	p.codes[code] = authorization{
		identity:      identity,
		nonce:         params["nonce"],
		codeChallenge: params["code_challenge"],
		redirectURI:   params["redirect_uri"],
	}
	p.mu.Unlock()
	p.redirect(w, r, params["redirect_uri"], url.Values{"code": {code}, "state": {params["state"]}})
}

func (p *Provider) redirect(w http.ResponseWriter, r *http.Request, target string, values url.Values) {
	u, err := url.Parse(target)
	if err != nil {
		http.Error(w, "invalid redirect_uri", http.StatusBadRequest)
		return
	}
	u.RawQuery = values.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

func (p *Provider) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	clientID, clientSecret, ok := r.BasicAuth()
	if !ok {
		clientID, clientSecret = r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	}
	if clientID != ClientID || clientSecret != ClientSecret {
		tokenError(w, "invalid_client")
		return
	}
	code := r.PostForm.Get("code")
	p.mu.Lock()
	auth, found := p.codes[code]
	delete(p.codes, code)
	p.mu.Unlock()
	if !found || auth.redirectURI != r.PostForm.Get("redirect_uri") {
		tokenError(w, "invalid_grant")
		return
	}
	verifier := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
	if base64.RawURLEncoding.EncodeToString(verifier[:]) != auth.codeChallenge {
		tokenError(w, "invalid_grant")
		return
	}
	idToken, err := p.sign(auth)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"access_token": randomString(),
		"token_type":   "Bearer",
		"expires_in":   3600,
		"id_token":     idToken,
	})
}

func (p *Provider) sign(auth authorization) (string, error) {
	now := p.now()
	claims := map[string]any{
		"iss":   p.Issuer,
		"aud":   ClientID,
		"sub":   auth.identity.Subject,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
		"nonce": auth.nonce,
	}
	if auth.identity.Email != "" {
		claims["email"] = auth.identity.Email
	}
	if auth.identity.Name != "" {
		claims["name"] = auth.identity.Name
	}
	if auth.identity.PreferredUsername != "" {
		claims["preferred_username"] = auth.identity.PreferredUsername
	}
	header, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": keyID})
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	signingInput := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, p.key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func tokenError(w http.ResponseWriter, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_, _ = fmt.Fprintf(w, `{"error":%q}`, code)
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func randomString() string {
	buf := make([]byte, 18)
	_, _ = rand.Read(buf)
	return base64.RawURLEncoding.EncodeToString(buf)
}
