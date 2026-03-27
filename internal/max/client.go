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
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		defer mw.Close()
		part, err := mw.CreateFormFile("data", safeFileName(fileName))
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		if _, err := io.Copy(part, content); err != nil {
			_ = pw.CloseWithError(err)
		}
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, pr)
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
	if strings.Contains(strings.ToLower(apiErr.Body), "attachment.not.ready") {
		return true
	}
	return false
}
