package httpapi

import (
	"testing"

	"maxbridge/internal/domain"
)

func TestExtractMaxLinkInput_FromMessageText(t *testing.T) {
	upd := domain.MaxWebhookUpdate{
		Message: &domain.MaxWebhookMessage{
			Text: "/link MB-ABC123",
			Sender: domain.MaxSenderRef{
				UserID:    1001,
				FirstName: "Ivan",
				LastName:  "Petrov",
			},
		},
	}

	userID, text, firstName, lastName, ok := extractMaxLinkInput(upd)
	if !ok {
		t.Fatal("expected input to be extracted")
	}
	if userID != 1001 {
		t.Fatalf("unexpected user id: %d", userID)
	}
	if text != "/link MB-ABC123" {
		t.Fatalf("unexpected text: %s", text)
	}
	if firstName != "Ivan" {
		t.Fatalf("unexpected first_name: %s", firstName)
	}
	if lastName != "Petrov" {
		t.Fatalf("unexpected last_name: %s", lastName)
	}
}

func TestExtractMaxLinkInput_FromBodyText(t *testing.T) {
	upd := domain.MaxWebhookUpdate{
		Message: &domain.MaxWebhookMessage{
			Body: &domain.MaxWebhookBody{
				Text: "/link MB-XYZ789",
			},
			Sender: domain.MaxSenderRef{
				UserID: 1002,
			},
		},
	}

	userID, text, firstName, lastName, ok := extractMaxLinkInput(upd)
	if !ok {
		t.Fatal("expected input to be extracted from body.text")
	}
	if userID != 1002 {
		t.Fatalf("unexpected user id: %d", userID)
	}
	if text != "/link MB-XYZ789" {
		t.Fatalf("unexpected text: %s", text)
	}
	if firstName != "" {
		t.Fatalf("unexpected first_name: %s", firstName)
	}
	if lastName != "" {
		t.Fatalf("unexpected last_name: %s", lastName)
	}
}

func TestExtractMaxLinkInput_InvalidWhenUserMissing(t *testing.T) {
	upd := domain.MaxWebhookUpdate{
		Message: &domain.MaxWebhookMessage{
			Text: "/link MB-ABC123",
		},
	}

	_, _, _, _, ok := extractMaxLinkInput(upd)
	if ok {
		t.Fatal("expected invalid input when user_id is missing")
	}
}
