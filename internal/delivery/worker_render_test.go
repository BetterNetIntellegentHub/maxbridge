package delivery

import (
	"encoding/json"
	"strings"
	"testing"

	"maxbridge/internal/domain"
)

func TestRenderMessageIncludesChatAndSenderAndText(t *testing.T) {
	msg := domain.TelegramMessage{
		MessageID: 77,
		Text:      "hello from tg",
		From: &domain.TelegramUser{
			FirstName: "Ivan",
			LastName:  "Petrov",
		},
		Chat: domain.TelegramChat{
			ID:    -10042,
			Title: "Ops Chat",
		},
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	out := renderMessage(domain.DeliveryJob{
		TelegramChatID:    msg.Chat.ID,
		TelegramMessageID: msg.MessageID,
		PayloadJSON:       payload,
	})

	if !strings.Contains(out.Text, "Ops Chat - Ivan Petrov") {
		t.Fatalf("expected header with chat and sender, got: %q", out.Text)
	}
	if !strings.Contains(out.Text, "hello from tg") {
		t.Fatalf("expected text in output, got: %q", out.Text)
	}
	if len(out.Media) != 0 {
		t.Fatalf("expected no media attachments, got: %d", len(out.Media))
	}
}

func TestRenderMessageIncludesMediaAttachments(t *testing.T) {
	msg := domain.TelegramMessage{
		MessageID: 81,
		Caption:   "see attachment",
		From: &domain.TelegramUser{
			Username: "bridge_user",
		},
		Chat: domain.TelegramChat{
			ID:    -1001,
			Title: "Media Chat",
		},
		Photo:    []domain.TelegramPhoto{{FileID: "photo-file-id"}},
		Document: &domain.TelegramDocument{FileName: "report.pdf"},
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	out := renderMessage(domain.DeliveryJob{
		TelegramChatID:    msg.Chat.ID,
		TelegramMessageID: msg.MessageID,
		PayloadJSON:       payload,
	})

	if !strings.Contains(out.Text, "Media Chat - @bridge_user") {
		t.Fatalf("expected media header, got: %q", out.Text)
	}
	if !strings.Contains(out.Text, "see attachment") {
		t.Fatalf("expected caption in output, got: %q", out.Text)
	}
	if len(out.Media) != 1 {
		t.Fatalf("expected 1 media attachment, got: %d", len(out.Media))
	}
	if out.Media[0].AttachmentType != "image" {
		t.Fatalf("expected image attachment type, got: %q", out.Media[0].AttachmentType)
	}
}
