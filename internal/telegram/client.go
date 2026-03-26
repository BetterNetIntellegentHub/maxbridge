package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"maxbridge/internal/domain"
)

type Client struct {
	token string
	http  *http.Client
}

type ProbeResult struct {
	Readiness domain.GroupReadiness
	Reason    string
	BotStatus string
	CanReadAll bool
}

type tgResponse[T any] struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
	Result      T      `json:"result"`
}

type getMeResult struct {
	ID                      int64 `json:"id"`
	CanReadAllGroupMessages bool  `json:"can_read_all_group_messages"`
}

type getChatMemberResult struct {
	Status string `json:"status"`
}

func NewClient(token string) *Client {
	return &Client{
		token: token,
		http:  &http.Client{Timeout: 6 * time.Second},
	}
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.getMe(ctx)
	return err
}

func (c *Client) ProbeGroup(ctx context.Context, chatID int64) (ProbeResult, error) {
	me, err := c.getMe(ctx)
	if err != nil {
		return ProbeResult{Readiness: domain.GroupBlocked, Reason: "telegram getMe failed"}, err
	}
	member, err := c.getChatMember(ctx, chatID, me.ID)
	if err != nil {
		return ProbeResult{Readiness: domain.GroupBlocked, Reason: "bot is not a group member or chat inaccessible"}, err
	}

	result := ProbeResult{
		BotStatus:   member.Status,
		CanReadAll:  me.CanReadAllGroupMessages,
		Readiness:   domain.GroupReady,
		Reason:      "all checks passed; smoke test requires a real user message in group",
	}

	switch member.Status {
	case "left", "kicked":
		result.Readiness = domain.GroupBlocked
		result.Reason = "bot is not active in this group"
		return result, nil
	case "member", "administrator":
		if !me.CanReadAllGroupMessages {
			result.Readiness = domain.GroupLimited
			result.Reason = "privacy mode may hide non-command messages; grant admin and disable privacy mode"
		}
	default:
		result.Readiness = domain.GroupLimited
		result.Reason = "unexpected bot member status"
	}

	return result, nil
}

func (c *Client) DeepLinkStartGroup(botUsername string, payload string) string {
	u := url.URL{Scheme: "https", Host: "t.me", Path: "/" + botUsername}
	q := u.Query()
	q.Set("startgroup", payload)
	u.RawQuery = q.Encode()
	return u.String()
}

func (c *Client) getMe(ctx context.Context) (getMeResult, error) {
	resp, err := c.call(ctx, "getMe", nil)
	if err != nil {
		return getMeResult{}, err
	}
	var parsed tgResponse[getMeResult]
	if err := json.Unmarshal(resp, &parsed); err != nil {
		return getMeResult{}, err
	}
	if !parsed.OK {
		return getMeResult{}, fmt.Errorf("telegram getMe not ok")
	}
	return parsed.Result, nil
}

func (c *Client) getChatMember(ctx context.Context, chatID, userID int64) (getChatMemberResult, error) {
	params := map[string]string{"chat_id": strconv.FormatInt(chatID, 10), "user_id": strconv.FormatInt(userID, 10)}
	resp, err := c.call(ctx, "getChatMember", params)
	if err != nil {
		return getChatMemberResult{}, err
	}
	var parsed tgResponse[getChatMemberResult]
	if err := json.Unmarshal(resp, &parsed); err != nil {
		return getChatMemberResult{}, err
	}
	if !parsed.OK {
		return getChatMemberResult{}, fmt.Errorf("telegram getChatMember not ok")
	}
	return parsed.Result, nil
}

func (c *Client) call(ctx context.Context, method string, query map[string]string) ([]byte, error) {
	u := fmt.Sprintf("https://api.telegram.org/bot%s/%s", c.token, method)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	if len(query) > 0 {
		q := req.URL.Query()
		for k, v := range query {
			q.Set(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("telegram api status=%d body=%s", resp.StatusCode, string(body))
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

