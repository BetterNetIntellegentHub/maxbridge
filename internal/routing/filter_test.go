package routing

import (
	"testing"

	"maxbridge/internal/domain"
)

func TestPassesRouteFilter(t *testing.T) {
	msg := &domain.TelegramMessage{Text: "hello", From: &domain.TelegramUser{IsBot: false}}
	if !PassesRouteFilter(domain.RouteFilterAll, false, msg) {
		t.Fatal("all should pass")
	}
	if !PassesRouteFilter(domain.RouteFilterTextOnly, false, msg) {
		t.Fatal("text_only should pass")
	}

	msg.Text = ""
	if PassesRouteFilter(domain.RouteFilterTextOnly, false, msg) {
		t.Fatal("text_only should fail for empty text")
	}
}
