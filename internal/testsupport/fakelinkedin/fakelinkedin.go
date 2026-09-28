// Package fakelinkedin imitates the parts of LinkedIn's API that postik uses,
// for tests and local development: the OAuth flow, the member and their
// pages, media uploads, posts and comments.
package fakelinkedin

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
)

// Picture is a valid 1×1 PNG, for avatars and logos that a browser can draw.
var Picture, _ = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=")

const (
	ClientID     = "fake-linkedin-client"
	ClientSecret = "fake-linkedin-secret"
	// PartSize is how LinkedIn splits a video upload.
	PartSize = 4 << 20
	// TokenLifetime is what LinkedIn gives an access token: 60 days.
	TokenLifetime = 60 * 24 * 60 * 60
)

// Page is an organization and the member's role in it.
type Page struct {
	ID         string
	Name       string
	VanityName string
	Role       string
	Logo       []byte
}

// Member is a LinkedIn account.
type Member struct {
	Sub     string
	Name    string
	Picture []byte
	Pages   []Page
	// NoRefreshTokens makes the token answers leave out the refresh token,
	// like an app without programmatic refresh tokens.
	NoRefreshTokens bool
}

// Post is what postik published.
type Post struct {
	URN         string   `json:"urn"`
	Author      string   `json:"author"`
	Commentary  string   `json:"commentary"`
	Media       []string `json:"media"`
	MultiImage  []string `json:"multiImage"`
	Title       string   `json:"title"`
	Version     string   `json:"version"`
	Protocol    string   `json:"protocol"`
	AccessToken string   `json:"-"`
}

// Comment is a comment postik published on a post.
type Comment struct {
	ID     string `json:"id"`
	Object string `json:"object"`
	Actor  string `json:"actor"`
	Text   string `json:"text"`
}

// Upload is a medium postik uploaded.
type Upload struct {
	URN       string
	Owner     string
	Kind      string
	Data      []byte
	Parts     int
	Finalized bool
}

type grant struct {
	sub      string
	scopes   []string
	redirect string
}

type failure struct {
	path    string
	status  int
	message string
	drop    bool
}

type Server struct {
	// Base is the public URL of the fake, used in the links it hands out.
	Base string

	mu            sync.Mutex
	members       map[string]*Member
	next          *Member
	deny          bool
	fewer         bool
	codes         map[string]grant
	tokens        map[string]string
	refreshes     map[string]string
	renewed       int
	rejectRefresh bool
	uploads       map[string]*Upload
	uploadIDs     map[string]string
	posts         []Post
	comments      []Comment
	failures      []failure
	omitID        bool
	counter       int
}

func New(base string) *Server {
	return &Server{
		Base:      base,
		members:   map[string]*Member{},
		codes:     map[string]grant{},
		tokens:    map[string]string{},
		refreshes: map[string]string{},
		uploads:   map[string]*Upload{},
		uploadIDs: map[string]string{},
	}
}

// SetMember adds or replaces an account.
func (s *Server) SetMember(m Member) {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := m
	s.members[m.Sub] = &copied
}

// SignInNext makes the next authorization approve at once as m, without
// showing the form.
func (s *Server) SignInNext(m Member) {
	s.SetMember(m)
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := m
	s.next = &copied
}

// DenyNext makes the next authorization come back cancelled.
func (s *Server) DenyNext() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deny = true
}

// GrantFewerNext makes the next authorization grant one scope less than asked.
func (s *Server) GrantFewerNext() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fewer = true
}

// RejectRefresh makes every refresh token invalid from now on.
func (s *Server) RejectRefresh() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rejectRefresh = true
}

// FailNext makes the next API call whose path starts with prefix answer
// status with message instead of doing its work.
func (s *Server) FailNext(prefix string, status int, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failures = append(s.failures, failure{path: prefix, status: status, message: message})
}

// DropNext makes the next API call whose path starts with prefix read the
// request and close the connection without answering.
func (s *Server) DropNext(prefix string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failures = append(s.failures, failure{path: prefix, drop: true})
}

// OmitIDNext makes the next post answer 201 without its ID, as if LinkedIn
// published and left the x-restli-id header out.
func (s *Server) OmitIDNext() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.omitID = true
}

func (s *Server) Posts() []Post {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.posts)
}

func (s *Server) Comments() []Comment {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.comments)
}

// Upload returns an uploaded medium by its URN.
func (s *Server) Upload(urn string) (Upload, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.uploads[urn]
	if !ok {
		return Upload{}, false
	}
	return *u, true
}

// Renewals counts the refresh grants that were honored.
func (s *Server) Renewals() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.renewed
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /oauth/v2/authorization", s.authorize)
	mux.HandleFunc("POST /oauth/v2/authorization", s.submitForm)
	mux.HandleFunc("POST /oauth/v2/accessToken", s.token)
	mux.HandleFunc("GET /pictures/{sub}", s.picture)
	mux.HandleFunc("GET /logos/{id}", s.logo)
	mux.HandleFunc("PUT /upload/{id}", s.api(s.upload))
	mux.HandleFunc("GET /v2/userinfo", s.api(s.userInfo))
	mux.HandleFunc("GET /rest/organizationAcls", s.api(s.acls))
	mux.HandleFunc("GET /v2/organizations/{id}", s.api(s.organization))
	mux.HandleFunc("POST /rest/images", s.api(s.initializeImage))
	mux.HandleFunc("POST /rest/videos", s.api(s.videoAction))
	mux.HandleFunc("GET /rest/videos/{urn}", s.api(s.video))
	mux.HandleFunc("POST /rest/posts", s.api(s.createPost))
	mux.HandleFunc("POST /rest/socialActions/{urn}/comments", s.api(s.createComment))
	mux.HandleFunc("POST /_control/fail", s.controlFail)
	mux.HandleFunc("GET /_control/posts", s.controlPosts)
	return mux
}

var form = template.Must(template.New("form").Parse(`<!doctype html>
<html><head><title>LinkedIn falso</title>
<style>body{font-family:sans-serif;display:flex;justify-content:center;padding-top:60px}
form{display:flex;flex-direction:column;gap:12px;width:360px}input{padding:8px}button{padding:10px;background:#0a66c2;color:#fff;border:0}</style>
</head><body><form method="post">
<h1>LinkedIn falso</h1>
<label>Subject <input name="sub" value="ruben-linkedin" required></label>
<label>Name <input name="name" value="Rubén García"></label>
<label>Pages <input name="pages" value="Zetesis, Nexo Labs"></label>
<label><input type="checkbox" name="refresh" value="yes" checked> Refresh tokens</label>
{{range $k, $v := .}}<input type="hidden" name="{{$k}}" value="{{$v}}">{{end}}
<button type="submit">Allow</button>
</form></body></html>`))

func (s *Server) authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("client_id") != ClientID || q.Get("response_type") != "code" {
		http.Error(w, "unknown client", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	next, deny := s.next, s.deny
	s.next, s.deny = nil, false
	s.mu.Unlock()
	params := map[string]string{"redirect_uri": q.Get("redirect_uri"), "state": q.Get("state"), "scope": q.Get("scope")}
	switch {
	case deny:
		s.redirect(w, r, params, url.Values{"error": {"user_cancelled_authorize"}, "error_description": {"The user cancelled the authorization"}})
	case next != nil:
		s.approve(w, r, next.Sub, params)
	default:
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = form.Execute(w, params)
	}
}

// submitForm creates the member typed in the form, with an administered page
// per name.
func (s *Server) submitForm(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	member := Member{Sub: r.PostForm.Get("sub"), Name: r.PostForm.Get("name"), Picture: Picture, NoRefreshTokens: r.PostForm.Get("refresh") != "yes"}
	for name := range strings.SplitSeq(r.PostForm.Get("pages"), ",") {
		if name = strings.TrimSpace(name); name != "" {
			member.Pages = append(member.Pages, PageNamed(name))
		}
	}
	s.SetMember(member)
	s.approve(w, r, member.Sub, map[string]string{"redirect_uri": r.PostForm.Get("redirect_uri"), "state": r.PostForm.Get("state"), "scope": r.PostForm.Get("scope")})
}

// PageNamed is an administered page whose ID and vanity name come from its name.
func PageNamed(name string) Page {
	sum := 0
	for _, r := range name {
		sum = sum*31 + int(r)
	}
	if sum < 0 {
		sum = -sum
	}
	return Page{ID: strconv.Itoa(1_000_000 + sum%9_000_000), Name: name, VanityName: strings.ToLower(strings.ReplaceAll(name, " ", "-")), Role: "ADMINISTRATOR", Logo: Picture}
}

func (s *Server) approve(w http.ResponseWriter, r *http.Request, sub string, params map[string]string) {
	scopes := strings.Fields(params["scope"])
	s.mu.Lock()
	if s.fewer && len(scopes) > 0 {
		scopes = scopes[:len(scopes)-1]
	}
	s.fewer = false
	s.counter++
	code := fmt.Sprintf("code-%d", s.counter)
	s.codes[code] = grant{sub: sub, scopes: scopes, redirect: params["redirect_uri"]}
	s.mu.Unlock()
	s.redirect(w, r, params, url.Values{"code": {code}})
}

func (s *Server) redirect(w http.ResponseWriter, r *http.Request, params map[string]string, values url.Values) {
	target, err := url.Parse(params["redirect_uri"])
	if err != nil {
		http.Error(w, "bad redirect_uri", http.StatusBadRequest)
		return
	}
	values.Set("state", params["state"])
	target.RawQuery = values.Encode()
	http.Redirect(w, r, target.String(), http.StatusFound)
}

func (s *Server) token(w http.ResponseWriter, r *http.Request) {
	if s.fail(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil || r.PostForm.Get("client_id") != ClientID || r.PostForm.Get("client_secret") != ClientSecret {
		oauthError(w, "invalid_client", "Client authentication failed")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var g grant
	var refresh string
	switch r.PostForm.Get("grant_type") {
	case "authorization_code":
		var ok bool
		g, ok = s.codes[r.PostForm.Get("code")]
		delete(s.codes, r.PostForm.Get("code"))
		if !ok || g.redirect != r.PostForm.Get("redirect_uri") {
			oauthError(w, "invalid_request", "Unable to retrieve access token: appid/redirect uri/code verifier does not match authorization code")
			return
		}
	case "refresh_token":
		refresh = r.PostForm.Get("refresh_token")
		sub, ok := s.refreshes[refresh]
		if !ok || s.rejectRefresh {
			oauthError(w, "invalid_grant", "The provided authorization grant is invalid, expired or revoked")
			return
		}
		g = grant{sub: sub}
		s.renewed++
	default:
		oauthError(w, "unsupported_grant_type", "Unsupported grant type")
		return
	}
	member := s.members[g.sub]
	s.counter++
	access := fmt.Sprintf("access-%s-%d", g.sub, s.counter)
	s.tokens[access] = g.sub
	answer := map[string]any{"access_token": access, "expires_in": TokenLifetime}
	if g.scopes != nil {
		answer["scope"] = strings.Join(g.scopes, ",")
	}
	if member != nil && !member.NoRefreshTokens {
		if refresh == "" {
			refresh = fmt.Sprintf("refresh-%s-%d", g.sub, s.counter)
			s.refreshes[refresh] = g.sub
		}
		answer["refresh_token"] = refresh
		answer["refresh_token_expires_in"] = 365 * 24 * 60 * 60
	}
	writeJSON(w, http.StatusOK, answer)
}

func oauthError(w http.ResponseWriter, code, description string) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": code, "error_description": description})
}

// api checks the access token and applies the prepared failures before
// handing the call to h with the member.
func (s *Server) api(h func(http.ResponseWriter, *http.Request, *Member, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.fail(w, r) {
			return
		}
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		s.mu.Lock()
		member := s.members[s.tokens[token]]
		s.mu.Unlock()
		if member == nil {
			restError(w, http.StatusUnauthorized, "EMPTY_ACCESS_TOKEN", "Invalid access token")
			return
		}
		h(w, r, member, token)
	}
}

func (s *Server) fail(w http.ResponseWriter, r *http.Request) bool {
	s.mu.Lock()
	var f *failure
	for i, candidate := range s.failures {
		if strings.HasPrefix(r.URL.Path, candidate.path) {
			f = &candidate
			s.failures = slices.Delete(s.failures, i, i+1)
			break
		}
	}
	s.mu.Unlock()
	if f == nil {
		return false
	}
	if f.drop {
		_, _ = io.Copy(io.Discard, r.Body)
		if hijacker, ok := w.(http.Hijacker); ok {
			if conn, _, err := hijacker.Hijack(); err == nil {
				_ = conn.Close()
				return true
			}
		}
		panic(http.ErrAbortHandler)
	}
	restError(w, f.status, "FAKE_FAILURE", f.message)
	return true
}

func restError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"status": status, "code": code, "message": message})
}

func (s *Server) userInfo(w http.ResponseWriter, _ *http.Request, m *Member, _ string) {
	writeJSON(w, http.StatusOK, map[string]string{"sub": m.Sub, "name": m.Name, "picture": s.Base + "/pictures/" + url.PathEscape(m.Sub), "email_verified": "true"})
}

func (s *Server) picture(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	m := s.members[r.PathValue("sub")]
	s.mu.Unlock()
	if m == nil || len(m.Picture) == 0 {
		http.NotFound(w, r)
		return
	}
	_, _ = w.Write(m.Picture)
}

func (s *Server) logo(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.members {
		for _, p := range m.Pages {
			if p.ID == r.PathValue("id") && len(p.Logo) > 0 {
				_, _ = w.Write(p.Logo)
				return
			}
		}
	}
	http.NotFound(w, r)
}

func (s *Server) acls(w http.ResponseWriter, r *http.Request, m *Member, _ string) {
	if r.URL.Query().Get("q") != "roleAssignee" {
		restError(w, http.StatusBadRequest, "BAD_QUERY", "q must be roleAssignee")
		return
	}
	elements := []map[string]string{}
	for _, p := range m.Pages {
		elements = append(elements, map[string]string{
			"organization": "urn:li:organization:" + p.ID, "role": p.Role, "state": "APPROVED", "roleAssignee": "urn:li:person:" + m.Sub,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"elements": elements})
}

func (s *Server) organization(w http.ResponseWriter, r *http.Request, _ *Member, _ string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.members {
		for _, p := range m.Pages {
			if p.ID != r.PathValue("id") {
				continue
			}
			id, _ := strconv.ParseInt(p.ID, 10, 64)
			writeJSON(w, http.StatusOK, map[string]any{
				"id": id, "localizedName": p.Name, "vanityName": p.VanityName,
				"logoV2": map[string]any{"original~": map[string]any{"elements": []any{
					map[string]any{"identifiers": []any{map[string]string{"identifier": s.Base + "/logos/" + p.ID}}},
				}}},
			})
			return
		}
	}
	restError(w, http.StatusNotFound, "NOT_FOUND", "Organization not found")
}

func (s *Server) newUpload(kind, owner string) *Upload {
	s.counter++
	id := strconv.Itoa(s.counter)
	u := &Upload{URN: fmt.Sprintf("urn:li:%s:C%s", kind, id), Owner: owner, Kind: kind}
	s.uploads[u.URN] = u
	s.uploadIDs[id] = u.URN
	return u
}

func (s *Server) initializeImage(w http.ResponseWriter, r *http.Request, _ *Member, _ string) {
	var body struct {
		Request struct {
			Owner string `json:"owner"`
		} `json:"initializeUploadRequest"`
	}
	if r.URL.Query().Get("action") != "initializeUpload" || json.NewDecoder(r.Body).Decode(&body) != nil || body.Request.Owner == "" {
		restError(w, http.StatusBadRequest, "BAD_REQUEST", "initializeUpload needs an owner")
		return
	}
	s.mu.Lock()
	u := s.newUpload("image", body.Request.Owner)
	id := strings.TrimPrefix(u.URN, "urn:li:image:C")
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"value": map[string]string{"uploadUrl": s.Base + "/upload/" + id, "image": u.URN}})
}

func (s *Server) videoAction(w http.ResponseWriter, r *http.Request, _ *Member, _ string) {
	switch r.URL.Query().Get("action") {
	case "initializeUpload":
		var body struct {
			Request struct {
				Owner string `json:"owner"`
				Size  int64  `json:"fileSizeBytes"`
			} `json:"initializeUploadRequest"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil || body.Request.Owner == "" || body.Request.Size <= 0 {
			restError(w, http.StatusBadRequest, "BAD_REQUEST", "initializeUpload needs an owner and a size")
			return
		}
		s.mu.Lock()
		u := s.newUpload("video", body.Request.Owner)
		id := strings.TrimPrefix(u.URN, "urn:li:video:C")
		s.mu.Unlock()
		var instructions []map[string]any
		for first := int64(0); first < body.Request.Size; first += PartSize {
			last := min(first+PartSize, body.Request.Size) - 1
			instructions = append(instructions, map[string]any{"uploadUrl": fmt.Sprintf("%s/upload/%s?part=%d", s.Base, id, len(instructions)), "firstByte": first, "lastByte": last})
		}
		writeJSON(w, http.StatusOK, map[string]any{"value": map[string]any{"video": u.URN, "uploadToken": "token-" + id, "uploadInstructions": instructions}})
	case "finalizeUpload":
		var body struct {
			Request struct {
				Video string   `json:"video"`
				Parts []string `json:"uploadedPartIds"`
			} `json:"finalizeUploadRequest"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			restError(w, http.StatusBadRequest, "BAD_REQUEST", "bad finalizeUpload")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		u, ok := s.uploads[body.Request.Video]
		if !ok || len(body.Request.Parts) != u.Parts || slices.Contains(body.Request.Parts, "") {
			restError(w, http.StatusBadRequest, "BAD_REQUEST", "the parts do not match the upload")
			return
		}
		u.Finalized = true
		w.WriteHeader(http.StatusOK)
	default:
		restError(w, http.StatusBadRequest, "BAD_REQUEST", "unknown action")
	}
}

func (s *Server) video(w http.ResponseWriter, r *http.Request, _ *Member, _ string) {
	s.mu.Lock()
	u, ok := s.uploads[r.PathValue("urn")]
	s.mu.Unlock()
	if !ok {
		restError(w, http.StatusNotFound, "NOT_FOUND", "Video not found")
		return
	}
	status := "WAITING_UPLOAD"
	if u.Finalized {
		status = "AVAILABLE"
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": u.URN, "status": status})
}

func (s *Server) upload(w http.ResponseWriter, r *http.Request, _ *Member, _ string) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.uploads[s.uploadIDs[r.PathValue("id")]]
	if !ok {
		http.NotFound(w, r)
		return
	}
	u.Data = append(u.Data, data...)
	u.Parts++
	w.Header().Set("ETag", fmt.Sprintf("etag-%s-%d", r.PathValue("id"), u.Parts))
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) createPost(w http.ResponseWriter, r *http.Request, m *Member, token string) {
	var body struct {
		Author     string `json:"author"`
		Commentary string `json:"commentary"`
		Content    struct {
			Media struct {
				ID    string `json:"id"`
				Title string `json:"title"`
			} `json:"media"`
			MultiImage struct {
				Images []struct {
					ID string `json:"id"`
				} `json:"images"`
			} `json:"multiImage"`
		} `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		restError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if !s.mayPostAs(m, body.Author) {
		restError(w, http.StatusForbidden, "ACCESS_DENIED", "Not enough permissions to post as "+body.Author)
		return
	}
	post := Post{Author: body.Author, Commentary: body.Commentary, Title: body.Content.Media.Title, Version: r.Header.Get("Linkedin-Version"), Protocol: r.Header.Get("X-Restli-Protocol-Version"), AccessToken: token}
	if body.Content.Media.ID != "" {
		post.Media = []string{body.Content.Media.ID}
	}
	for _, image := range body.Content.MultiImage.Images {
		post.MultiImage = append(post.MultiImage, image.ID)
	}
	s.mu.Lock()
	s.counter++
	post.URN = fmt.Sprintf("urn:li:share:%d", 7_000_000_000+s.counter)
	s.posts = append(s.posts, post)
	omit := s.omitID
	s.omitID = false
	s.mu.Unlock()
	if !omit {
		w.Header().Set("x-restli-id", post.URN)
	}
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) mayPostAs(m *Member, author string) bool {
	if author == "urn:li:person:"+m.Sub {
		return true
	}
	for _, p := range m.Pages {
		if author == "urn:li:organization:"+p.ID && p.Role == "ADMINISTRATOR" {
			return true
		}
	}
	return false
}

func (s *Server) createComment(w http.ResponseWriter, r *http.Request, m *Member, _ string) {
	var body struct {
		Actor   string `json:"actor"`
		Object  string `json:"object"`
		Message struct {
			Text string `json:"text"`
		} `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Object != r.PathValue("urn") {
		restError(w, http.StatusBadRequest, "BAD_REQUEST", "the object must be the post in the path")
		return
	}
	if !s.mayPostAs(m, body.Actor) {
		restError(w, http.StatusForbidden, "ACCESS_DENIED", "Not enough permissions to comment as "+body.Actor)
		return
	}
	s.mu.Lock()
	s.counter++
	comment := Comment{ID: strconv.Itoa(8_000_000_000 + s.counter), Object: body.Object, Actor: body.Actor, Text: body.Message.Text}
	s.comments = append(s.comments, comment)
	s.mu.Unlock()
	w.Header().Set("x-restli-id", comment.ID)
	w.WriteHeader(http.StatusCreated)
}

// controlFail prepares a failure: {"path": "/rest/posts", "status": 401,
// "message": "…"} or {"path": "…", "drop": true}.
func (s *Server) controlFail(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path    string `json:"path"`
		Status  int    `json:"status"`
		Message string `json:"message"`
		Drop    bool   `json:"drop"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if body.Drop {
		s.DropNext(body.Path)
	} else {
		s.FailNext(body.Path, body.Status, body.Message)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) controlPosts(w http.ResponseWriter, _ *http.Request) {
	posts := s.Posts()
	if posts == nil {
		posts = []Post{}
	}
	writeJSON(w, http.StatusOK, posts)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
