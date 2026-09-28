// Package linkedin is a small client for the parts of LinkedIn's API that
// postik uses: the OAuth flow, the member and their pages, media uploads,
// posts and comments.
package linkedin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var (
	// MemberScopes let postik publish as the member who authorizes.
	MemberScopes = []string{"openid", "profile", "w_member_social"}
	// PageScopes let postik list the pages the member administers and publish as them.
	PageScopes = []string{"openid", "profile", "r_organization_social", "w_organization_social", "rw_organization_admin"}
)

type Client struct {
	AuthURL      string
	APIURL       string
	ClientID     string
	ClientSecret string
	Version      string
	HTTP         *http.Client
	// Uploads sends files, which can take much longer than a plain call.
	Uploads *http.Client
	// PollInterval and VideoWait bound the wait for a video to be processed.
	PollInterval time.Duration
	VideoWait    time.Duration
}

func New(authURL, apiURL, clientID, clientSecret, version string) *Client {
	return &Client{
		AuthURL:      authURL,
		APIURL:       apiURL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Version:      version,
		HTTP:         &http.Client{Timeout: 30 * time.Second},
		Uploads:      &http.Client{Timeout: 10 * time.Minute},
		PollInterval: 5 * time.Second,
		VideoWait:    10 * time.Minute,
	}
}

// APIError is an answer of LinkedIn with an error status.
type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("linkedin %d: %s", e.Status, e.Message)
}

// NotSentError means the request never reached LinkedIn: the connection
// could not be opened, so nothing can have been published.
type NotSentError struct {
	Err error
}

func (e *NotSentError) Error() string { return "linkedin unreachable: " + e.Err.Error() }
func (e *NotSentError) Unwrap() error { return e.Err }

func notSent(err error) bool {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}
	var opErr *net.OpError
	return errors.As(err, &opErr) && opErr.Op == "dial"
}

// AuthorizeURL is where the browser goes to grant the scopes.
func (c *Client) AuthorizeURL(state, redirect string, scopes []string) string {
	q := url.Values{
		"response_type": {"code"},
		"client_id":     {c.ClientID},
		"redirect_uri":  {redirect},
		"state":         {state},
		"scope":         {strings.Join(scopes, " ")},
	}
	return c.AuthURL + "/oauth/v2/authorization?" + q.Encode()
}

type Token struct {
	AccessToken           string `json:"access_token"`
	ExpiresIn             int64  `json:"expires_in"`
	RefreshToken          string `json:"refresh_token"`
	RefreshTokenExpiresIn int64  `json:"refresh_token_expires_in"`
	Scope                 string `json:"scope"`
}

func (c *Client) Exchange(ctx context.Context, code, redirect string) (Token, error) {
	return c.token(ctx, url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {redirect}})
}

func (c *Client) Refresh(ctx context.Context, refreshToken string) (Token, error) {
	return c.token(ctx, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refreshToken}})
}

func (c *Client) token(ctx context.Context, form url.Values) (Token, error) {
	form.Set("client_id", c.ClientID)
	form.Set("client_secret", c.ClientSecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.AuthURL+"/oauth/v2/accessToken", strings.NewReader(form.Encode()))
	if err != nil {
		return Token{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var token Token
	_, err = c.do(c.HTTP, req, &token)
	return token, err
}

type UserInfo struct {
	Sub     string `json:"sub"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

func (c *Client) UserInfo(ctx context.Context, accessToken string) (UserInfo, error) {
	var info UserInfo
	err := c.get(ctx, accessToken, c.APIURL+"/v2/userinfo", &info)
	return info, err
}

// ACL is a role of the member in an organization.
type ACL struct {
	Organization string `json:"organization"`
	Role         string `json:"role"`
	State        string `json:"state"`
}

// ACLs lists the roles of the member in organizations.
func (c *Client) ACLs(ctx context.Context, accessToken string) ([]ACL, error) {
	var out []ACL
	for start := 0; ; start += 100 {
		var page struct {
			Elements []ACL `json:"elements"`
		}
		target := fmt.Sprintf("%s/rest/organizationAcls?q=roleAssignee&state=APPROVED&count=100&start=%d", c.APIURL, start)
		if err := c.get(ctx, accessToken, target, &page); err != nil {
			return nil, err
		}
		out = append(out, page.Elements...)
		if len(page.Elements) < 100 {
			return out, nil
		}
	}
}

type Organization struct {
	ID         string
	Name       string
	VanityName string
	LogoURL    string
}

// Organization reads the name and logo of a page.
func (c *Client) Organization(ctx context.Context, accessToken, id string) (Organization, error) {
	var raw struct {
		ID            json.Number `json:"id"`
		LocalizedName string      `json:"localizedName"`
		VanityName    string      `json:"vanityName"`
		LogoV2        struct {
			Original struct {
				Elements []struct {
					Identifiers []struct {
						Identifier string `json:"identifier"`
					} `json:"identifiers"`
				} `json:"elements"`
			} `json:"original~"`
		} `json:"logoV2"`
	}
	target := c.APIURL + "/v2/organizations/" + url.PathEscape(id) + "?projection=(id,localizedName,vanityName,logoV2(original~:playableStreams))"
	if err := c.get(ctx, accessToken, target, &raw); err != nil {
		return Organization{}, err
	}
	org := Organization{ID: raw.ID.String(), Name: raw.LocalizedName, VanityName: raw.VanityName}
	if elements := raw.LogoV2.Original.Elements; len(elements) > 0 {
		if identifiers := elements[len(elements)-1].Identifiers; len(identifiers) > 0 {
			org.LogoURL = identifiers[0].Identifier
		}
	}
	return org, nil
}

// maxPicture bounds the avatars and logos postik downloads.
const maxPicture = 5 << 20

// Download fetches a picture of LinkedIn's CDN.
func (c *Client) Download(ctx context.Context, target string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: status %d", target, res.StatusCode)
	}
	return io.ReadAll(io.LimitReader(res.Body, maxPicture))
}

// File is a medium on disk.
type File struct {
	Open func() (*os.File, error)
}

// UploadImage uploads an image for owner and returns its URN.
func (c *Client) UploadImage(ctx context.Context, accessToken, owner string, file File) (string, error) {
	var init struct {
		Value struct {
			UploadURL string `json:"uploadUrl"`
			Image     string `json:"image"`
		} `json:"value"`
	}
	body := map[string]any{"initializeUploadRequest": map[string]any{"owner": owner}}
	if _, err := c.rest(ctx, accessToken, http.MethodPost, "/rest/images?action=initializeUpload", body, &init); err != nil {
		return "", err
	}
	f, err := file.Open()
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if _, err := c.put(ctx, accessToken, init.Value.UploadURL, io.NewSectionReader(f, 0, info.Size()), info.Size()); err != nil {
		return "", err
	}
	return init.Value.Image, nil
}

// UploadVideo uploads a video in the parts LinkedIn asks for, finalizes it
// and waits until it can be published. It returns its URN.
func (c *Client) UploadVideo(ctx context.Context, accessToken, owner string, file File) (string, error) {
	f, err := file.Open()
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	var init struct {
		Value struct {
			Video              string `json:"video"`
			UploadToken        string `json:"uploadToken"`
			UploadInstructions []struct {
				UploadURL string `json:"uploadUrl"`
				FirstByte int64  `json:"firstByte"`
				LastByte  int64  `json:"lastByte"`
			} `json:"uploadInstructions"`
		} `json:"value"`
	}
	body := map[string]any{"initializeUploadRequest": map[string]any{
		"owner": owner, "fileSizeBytes": info.Size(), "uploadCaptions": false, "uploadThumbnail": false,
	}}
	if _, err := c.rest(ctx, accessToken, http.MethodPost, "/rest/videos?action=initializeUpload", body, &init); err != nil {
		return "", err
	}
	etags := make([]string, 0, len(init.Value.UploadInstructions))
	for _, part := range init.Value.UploadInstructions {
		size := part.LastByte - part.FirstByte + 1
		etag, err := c.put(ctx, accessToken, part.UploadURL, io.NewSectionReader(f, part.FirstByte, size), size)
		if err != nil {
			return "", err
		}
		etags = append(etags, etag)
	}
	finalize := map[string]any{"finalizeUploadRequest": map[string]any{
		"video": init.Value.Video, "uploadToken": init.Value.UploadToken, "uploadedPartIds": etags,
	}}
	if _, err := c.rest(ctx, accessToken, http.MethodPost, "/rest/videos?action=finalizeUpload", finalize, nil); err != nil {
		return "", err
	}
	return init.Value.Video, c.waitForVideo(ctx, accessToken, init.Value.Video)
}

func (c *Client) waitForVideo(ctx context.Context, accessToken, urn string) error {
	deadline := time.Now().Add(c.VideoWait)
	for {
		var video struct {
			Status string `json:"status"`
		}
		if _, err := c.rest(ctx, accessToken, http.MethodGet, "/rest/videos/"+EscapeURN(urn), nil, &video); err != nil {
			return err
		}
		switch video.Status {
		case "AVAILABLE":
			return nil
		case "PROCESSING_FAILED":
			return &APIError{Status: http.StatusUnprocessableEntity, Code: "PROCESSING_FAILED", Message: "LinkedIn could not process the video"}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("video %s is still %s", urn, video.Status)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(c.PollInterval):
		}
	}
}

// Content is what a post shows besides its text.
type Content struct {
	// Media are the URNs of the uploaded media. One goes as media, several as
	// a multi-image post.
	Media []string
	// Title names a document.
	Title string
}

type Post struct {
	Author     string
	Commentary string
	Content    Content
}

// CreatePost publishes and returns the post's URN.
func (c *Client) CreatePost(ctx context.Context, accessToken string, p Post) (string, error) {
	body := map[string]any{
		"author":                    p.Author,
		"commentary":                p.Commentary,
		"visibility":                "PUBLIC",
		"distribution":              map[string]any{"feedDistribution": "MAIN_FEED", "targetEntities": []any{}, "thirdPartyDistributionChannels": []any{}},
		"lifecycleState":            "PUBLISHED",
		"isReshareDisabledByAuthor": false,
	}
	switch len(p.Content.Media) {
	case 0:
	case 1:
		media := map[string]any{"id": p.Content.Media[0]}
		if p.Content.Title != "" {
			media["title"] = p.Content.Title
		}
		body["content"] = map[string]any{"media": media}
	default:
		images := make([]map[string]any, len(p.Content.Media))
		for i, urn := range p.Content.Media {
			images[i] = map[string]any{"id": urn}
		}
		body["content"] = map[string]any{"multiImage": map[string]any{"images": images}}
	}
	header, err := c.rest(ctx, accessToken, http.MethodPost, "/rest/posts", body, nil)
	if err != nil {
		return "", err
	}
	return header.Get("x-restli-id"), nil
}

// CreateComment comments on a post as actor and returns the comment's ID.
func (c *Client) CreateComment(ctx context.Context, accessToken, postURN, actor, text string) (string, error) {
	body := map[string]any{"actor": actor, "object": postURN, "message": map[string]any{"text": text}}
	header, err := c.rest(ctx, accessToken, http.MethodPost, "/rest/socialActions/"+EscapeURN(postURN)+"/comments", body, nil)
	if err != nil {
		return "", err
	}
	return header.Get("x-restli-id"), nil
}

// EscapeURN encodes a URN for a path segment, as Rest.li expects.
func EscapeURN(urn string) string {
	return strings.NewReplacer(":", "%3A", "(", "%28", ")", "%29", ",", "%2C").Replace(urn)
}

// get reads target. Only the versioned API (/rest) takes the version
// headers; the older /v2 endpoints get the token alone, as Postiz calls them.
func (c *Client) get(ctx context.Context, accessToken, target string, result any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	if strings.HasPrefix(target, c.APIURL+"/rest/") {
		c.authorize(req, accessToken)
	} else {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	_, err = c.do(c.HTTP, req, result)
	return err
}

func (c *Client) rest(ctx context.Context, accessToken, method, path string, body, result any) (http.Header, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.APIURL+path, reader)
	if err != nil {
		return nil, err
	}
	c.authorize(req, accessToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.do(c.HTTP, req, result)
}

func (c *Client) authorize(req *http.Request, accessToken string) {
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Linkedin-Version", c.Version)
	req.Header.Set("X-Restli-Protocol-Version", "2.0.0")
}

// put uploads a part and returns its ETag.
func (c *Client) put(ctx context.Context, accessToken, target string, body io.Reader, size int64) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, target, body)
	if err != nil {
		return "", err
	}
	req.ContentLength = size
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/octet-stream")
	header, err := c.do(c.Uploads, req, nil)
	if err != nil {
		return "", err
	}
	return header.Get("ETag"), nil
}

func (c *Client) do(client *http.Client, req *http.Request, result any) (http.Header, error) {
	res, err := client.Do(req)
	if err != nil {
		if notSent(err) {
			return nil, &NotSentError{Err: err}
		}
		return nil, fmt.Errorf("linkedin %s %s: %w", req.Method, req.URL.Path, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("linkedin %s %s: read: %w", req.Method, req.URL.Path, err)
	}
	if res.StatusCode >= 300 {
		return nil, apiError(res.StatusCode, body)
	}
	if result != nil && len(body) > 0 {
		if err := json.Unmarshal(body, result); err != nil {
			return nil, fmt.Errorf("linkedin %s %s: decode: %w", req.Method, req.URL.Path, err)
		}
	}
	return res.Header, nil
}

// apiError reads both shapes of LinkedIn errors: the API's (message, code)
// and the OAuth server's (error, error_description).
func apiError(status int, body []byte) *APIError {
	var raw struct {
		Message          string `json:"message"`
		Code             string `json:"code"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	_ = json.Unmarshal(body, &raw)
	e := &APIError{Status: status, Code: raw.Code, Message: raw.Message}
	if e.Code == "" {
		e.Code = raw.Error
	}
	if e.Message == "" {
		e.Message = raw.ErrorDescription
	}
	if e.Message == "" {
		e.Message = strings.TrimSpace(string(body))
	}
	if e.Message == "" {
		e.Message = http.StatusText(status)
	}
	return e
}
