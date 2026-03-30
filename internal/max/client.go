package max

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

type SendMessageRequest struct {
	Text        string       `json:"text"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

type Attachment struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload"`
}

type uploadInitResponse struct {
	URL   string `json:"url"`
	Token string `json:"token,omitempty"`
}

type APIError struct {
	StatusCode int
	Body       string
}

type UserProfile struct {
	FirstName  string
	LastName   string
	LastAccess time.Time
}

func (e *APIError) Error() string {
	return fmt.Sprintf("max api status=%d body=%s", e.StatusCode, e.Body)
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		http: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func normalizeAuthToken(token string) string {
	t := strings.TrimSpace(token)
	t = strings.TrimPrefix(t, "Bearer ")
	t = strings.TrimPrefix(t, "bearer ")
	return t
}

func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/subscriptions", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", normalizeAuthToken(c.token))
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 500 {
		return nil
	}
	bs, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	return &APIError{StatusCode: resp.StatusCode, Body: string(bs)}
}

func (c *Client) SendMessage(ctx context.Context, userID int64, text string) error {
	return c.SendMessageWithAttachments(ctx, userID, text, nil)
}

func (c *Client) SendMessageWithAttachments(ctx context.Context, userID int64, text string, attachments []Attachment) error {
	payload := SendMessageRequest{Text: text, Attachments: attachments}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	u := c.baseURL + "/messages?user_id=" + url.QueryEscape(fmt.Sprintf("%d", userID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", normalizeAuthToken(c.token))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	bs, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	return &APIError{StatusCode: resp.StatusCode, Body: string(bs)}
}

func (c *Client) UploadAttachment(ctx context.Context, uploadType string, fileName string, content io.Reader) (Attachment, error) {
	uploadURL, prefetchedToken, err := c.requestUploadURL(ctx, uploadType)
	if err != nil {
		return Attachment{}, err
	}
	payload, err := c.uploadFile(ctx, uploadURL, fileName, content)
	if err != nil {
		return Attachment{}, err
	}
	if prefetchedToken != "" {
		if _, ok := payload["token"]; !ok {
			payload["token"] = prefetchedToken
		}
	}
	return Attachment{Type: uploadType, Payload: payload}, nil
}

func (c *Client) requestUploadURL(ctx context.Context, uploadType string) (string, string, error) {
	u := c.baseURL + "/uploads?type=" + url.QueryEscape(uploadType)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", normalizeAuthToken(c.token))

	resp, err := c.http.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		bs, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", "", &APIError{StatusCode: resp.StatusCode, Body: string(bs)}
	}

	var out uploadInitResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return "", "", err
	}
	if strings.TrimSpace(out.URL) == "" {
		return "", "", fmt.Errorf("max uploads returned empty url")
	}
	return out.URL, strings.TrimSpace(out.Token), nil
}

func (c *Client) uploadFile(ctx context.Context, uploadURL string, fileName string, content io.Reader) (map[string]any, error) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("data", safeFileName(fileName))
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, content); err != nil {
		return nil, err
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, bytes.NewReader(body.Bytes()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", normalizeAuthToken(c.token))
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		bs, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, &APIError{StatusCode: resp.StatusCode, Body: string(bs)}
	}

	var payload map[string]any
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func safeFileName(name string) string {
	base := strings.TrimSpace(filepath.Base(name))
	if base == "." || base == "/" || base == "" {
		return "attachment.bin"
	}
	return base
}

func IsTemporarySendError(err error) bool {
	if err == nil {
		return false
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		return true
	}
	if apiErr.StatusCode == http.StatusTooManyRequests {
		return true
	}
	if apiErr.StatusCode >= 500 {
		return true
	}
	if IsAttachmentNotReadyError(err) {
		return true
	}
	return false
}

func IsAttachmentNotReadyError(err error) bool {
	apiErr, ok := err.(*APIError)
	if !ok {
		return false
	}
	body := strings.ToLower(apiErr.Body)
	return strings.Contains(body, "attachment.not.ready") || strings.Contains(body, "file.not.processed")
}

func (c *Client) LookupUserProfile(ctx context.Context, userID int64) (UserProfile, bool, error) {
	marker := ""
	var best UserProfile
	bestSet := false

	for page := 0; page < 20; page++ {
		chats, nextMarker, err := c.listChats(ctx, marker, 100)
		if err != nil {
			return UserProfile{}, false, err
		}
		for _, chatID := range chats {
			profile, found, err := c.getChatMember(ctx, chatID, userID)
			if err != nil {
				apiErr, ok := err.(*APIError)
				if ok && apiErr.StatusCode == http.StatusNotFound {
					continue
				}
				return UserProfile{}, false, err
			}
			if !found || strings.TrimSpace(profile.FirstName) == "" {
				continue
			}
			if !bestSet || (!profile.LastAccess.IsZero() && profile.LastAccess.After(best.LastAccess)) {
				best = profile
				bestSet = true
			}
		}
		if strings.TrimSpace(nextMarker) == "" || nextMarker == marker {
			break
		}
		marker = nextMarker
	}

	if !bestSet {
		return UserProfile{}, false, nil
	}
	return best, true, nil
}

func (c *Client) listChats(ctx context.Context, marker string, count int) ([]int64, string, error) {
	q := url.Values{}
	if strings.TrimSpace(marker) != "" {
		q.Set("marker", strings.TrimSpace(marker))
	}
	if count > 0 {
		q.Set("count", strconv.Itoa(count))
	}
	u := c.baseURL + "/chats"
	if qs := q.Encode(); qs != "" {
		u += "?" + qs
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Authorization", normalizeAuthToken(c.token))

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		bs, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, "", &APIError{StatusCode: resp.StatusCode, Body: string(bs)}
	}

	var payload any
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&payload); err != nil {
		return nil, "", err
	}
	return extractChats(payload)
}

func (c *Client) getChatMember(ctx context.Context, chatID int64, userID int64) (UserProfile, bool, error) {
	u := fmt.Sprintf("%s/chats/%d/members?user_ids=%d", c.baseURL, chatID, userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return UserProfile{}, false, err
	}
	req.Header.Set("Authorization", normalizeAuthToken(c.token))

	resp, err := c.http.Do(req)
	if err != nil {
		return UserProfile{}, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		bs, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return UserProfile{}, false, &APIError{StatusCode: resp.StatusCode, Body: string(bs)}
	}

	var payload any
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&payload); err != nil {
		return UserProfile{}, false, err
	}
	return extractMemberProfile(payload, userID)
}

func extractChats(payload any) ([]int64, string, error) {
	root, ok := payload.(map[string]any)
	if !ok {
		return nil, "", fmt.Errorf("max chats: unexpected response shape")
	}
	arr := pickArray(root, "chats", "items", "data")
	chats := make([]int64, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if id, ok := int64FromAny(m["chat_id"]); ok && id > 0 {
			chats = append(chats, id)
			continue
		}
		if id, ok := int64FromAny(m["id"]); ok && id > 0 {
			chats = append(chats, id)
		}
	}
	next := strings.TrimSpace(stringFromAny(root["marker"]))
	if next == "" {
		next = strings.TrimSpace(stringFromAny(root["next_marker"]))
	}
	return chats, next, nil
}

func extractMemberProfile(payload any, targetUserID int64) (UserProfile, bool, error) {
	items := pickMembers(payload)
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id, ok := int64FromAny(m["user_id"])
		if !ok {
			id, ok = int64FromAny(m["id"])
		}
		if !ok || id != targetUserID {
			continue
		}
		first := strings.TrimSpace(stringFromAny(m["first_name"]))
		last := strings.TrimSpace(stringFromAny(m["last_name"]))
		lastAccess := parseTimeAny(m["last_access_time"])
		return UserProfile{FirstName: first, LastName: last, LastAccess: lastAccess}, true, nil
	}
	return UserProfile{}, false, nil
}

func pickMembers(payload any) []any {
	if arr, ok := payload.([]any); ok {
		return arr
	}
	root, ok := payload.(map[string]any)
	if !ok {
		return nil
	}
	return pickArray(root, "members", "items", "data")
}

func pickArray(root map[string]any, keys ...string) []any {
	for _, key := range keys {
		if raw, ok := root[key]; ok {
			if arr, ok := raw.([]any); ok {
				return arr
			}
		}
	}
	return nil
}

func int64FromAny(v any) (int64, bool) {
	switch t := v.(type) {
	case float64:
		return int64(t), true
	case float32:
		return int64(t), true
	case int64:
		return t, true
	case int32:
		return int64(t), true
	case int:
		return int64(t), true
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
		if err != nil {
			return 0, false
		}
		return n, true
	default:
		return 0, false
	}
}

func stringFromAny(v any) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		return ""
	}
}

func parseTimeAny(v any) time.Time {
	s := strings.TrimSpace(stringFromAny(v))
	if s == "" {
		return time.Time{}
	}
	if tm, err := time.Parse(time.RFC3339, s); err == nil {
		return tm
	}
	return time.Time{}
}
