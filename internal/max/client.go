package max

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

type SendMessageRequest struct {
	UserID int64  `json:"user_id"`
	Text   string `json:"text"`
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
			Timeout: 5 * time.Second,
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
	payload := SendMessageRequest{UserID: userID, Text: text}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/messages", bytes.NewReader(body))
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
	return false
}
