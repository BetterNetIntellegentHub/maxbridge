package invites

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"maxbridge/internal/storage"
)

type Service struct {
	store  *storage.Store
	pepper string
}

type CreateInviteInput struct {
	ScopeType string
	ScopeID   string
	TTL       time.Duration
	SingleUse bool
	Metadata  map[string]any
}

type CreateInviteOutput struct {
	InviteID int64
	RawCode  string
	Expires  time.Time
}

func NewService(store *storage.Store, pepper string) *Service {
	return &Service{store: store, pepper: pepper}
}

func (s *Service) CreateInvite(ctx context.Context, input CreateInviteInput) (CreateInviteOutput, error) {
	if input.ScopeType == "" || input.ScopeID == "" {
		return CreateInviteOutput{}, errors.New("scope is required")
	}
	if input.TTL <= 0 {
		input.TTL = 24 * time.Hour
	}
	code, err := newCode()
	if err != nil {
		return CreateInviteOutput{}, err
	}
	hash := s.HashCode(code)
	expires := time.Now().UTC().Add(input.TTL)
	id, err := s.store.CreateInvite(
		ctx,
		input.ScopeType,
		input.ScopeID,
		hash,
		expires,
		input.SingleUse,
		input.Metadata,
	)
	if err != nil {
		return CreateInviteOutput{}, err
	}
	return CreateInviteOutput{InviteID: id, RawCode: code, Expires: expires}, nil
}

func (s *Service) HashCode(code string) string {
	norm := strings.TrimSpace(strings.ToUpper(code))
	sum := sha256.Sum256([]byte(norm + ":" + s.pepper))
	return hex.EncodeToString(sum[:])
}

func ParseLinkCommand(text string) (code string, ok bool) {
	parts := strings.Fields(strings.TrimSpace(text))
	if len(parts) != 2 {
		return "", false
	}
	if parts[0] != "/link" {
		return "", false
	}
	if len(parts[1]) < 6 {
		return "", false
	}
	return strings.ToUpper(parts[1]), true
}

func newCode() (string, error) {
	raw := make([]byte, 10)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	enc := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
	return fmt.Sprintf("MB-%s", enc[:12]), nil
}
