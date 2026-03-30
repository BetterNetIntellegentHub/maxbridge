package max

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLookupUserProfile_SuccessByLatestAccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/chats":
			_, _ = w.Write([]byte(`{"chats":[{"chat_id":10},{"chat_id":20}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/chats/10/members":
			_, _ = w.Write([]byte(`{"members":[{"user_id":234,"first_name":"Ivan","last_name":"Petrov","last_access_time":"2026-03-20T10:00:00Z"}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/chats/20/members":
			_, _ = w.Write([]byte(`{"members":[{"user_id":234,"first_name":"Ivan","last_name":"Sidorov","last_access_time":"2026-03-21T10:00:00Z"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "test-token")
	profile, found, err := c.LookupUserProfile(context.Background(), 234)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatalf("expected profile to be found")
	}
	if profile.FirstName != "Ivan" || profile.LastName != "Sidorov" {
		t.Fatalf("unexpected profile: %+v", profile)
	}
}

func TestLookupUserProfile_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/chats":
			_, _ = w.Write([]byte(`{"chats":[{"chat_id":10}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/chats/10/members":
			_, _ = w.Write([]byte(`{"members":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "test-token")
	_, found, err := c.LookupUserProfile(context.Background(), 999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatalf("expected no profile")
	}
}

func TestLookupUserProfile_TemporaryAPIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/chats" {
			http.Error(w, "temporary", http.StatusBadGateway)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "test-token")
	_, _, err := c.LookupUserProfile(context.Background(), 234)
	if err == nil {
		t.Fatalf("expected error")
	}
}
