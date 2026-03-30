package domain

import "time"

type GroupReadiness string

const (
	GroupReady   GroupReadiness = "READY"
	GroupLimited GroupReadiness = "LIMITED"
	GroupBlocked GroupReadiness = "BLOCKED"
)

type RouteFilterMode string

const (
	RouteFilterAll      RouteFilterMode = "all"
	RouteFilterTextOnly RouteFilterMode = "text_only"
	RouteFilterMentions RouteFilterMode = "mentions_only"
)

type DeliveryJobStatus string

const (
	JobPending    DeliveryJobStatus = "pending"
	JobProcessing DeliveryJobStatus = "processing"
	JobRetry      DeliveryJobStatus = "retry"
	JobCompleted  DeliveryJobStatus = "completed"
	JobDeadLetter DeliveryJobStatus = "dead_letter"
)

type DeliveryResult string

const (
	DeliverySuccess   DeliveryResult = "success"
	DeliveryTemporary DeliveryResult = "temporary_error"
	DeliveryPermanent DeliveryResult = "permanent_error"
)

type TelegramUpdate struct {
	UpdateID      int64                      `json:"update_id"`
	Message       *TelegramMessage           `json:"message,omitempty"`
	MyChatMember  *TelegramChatMemberUpdated `json:"my_chat_member,omitempty"`
	ChatMember    *TelegramChatMemberUpdated `json:"chat_member,omitempty"`
	ChannelPost   *TelegramMessage           `json:"channel_post,omitempty"`
	EditedMessage *TelegramMessage           `json:"edited_message,omitempty"`
}

type TelegramMessage struct {
	MessageID int64              `json:"message_id"`
	Date      int64              `json:"date"`
	Text      string             `json:"text,omitempty"`
	Caption   string             `json:"caption,omitempty"`
	From      *TelegramUser      `json:"from,omitempty"`
	Chat      TelegramChat       `json:"chat"`
	Entities  []TelegramEntity   `json:"entities,omitempty"`
	Photo     []TelegramPhoto    `json:"photo,omitempty"`
	Document  *TelegramDocument  `json:"document,omitempty"`
	Video     *TelegramVideo     `json:"video,omitempty"`
	Audio     *TelegramAudio     `json:"audio,omitempty"`
	Voice     *TelegramVoice     `json:"voice,omitempty"`
	Animation *TelegramAnimation `json:"animation,omitempty"`
}

type TelegramEntity struct {
	Type string `json:"type"`
}

type TelegramUser struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

type TelegramChat struct {
	ID       int64  `json:"id"`
	Title    string `json:"title,omitempty"`
	Username string `json:"username,omitempty"`
	Type     string `json:"type"`
}

type TelegramPhoto struct {
	FileID string `json:"file_id,omitempty"`
}

type TelegramDocument struct {
	FileID   string `json:"file_id,omitempty"`
	FileName string `json:"file_name,omitempty"`
}

type TelegramVideo struct {
	FileID string `json:"file_id,omitempty"`
}

type TelegramAudio struct {
	FileID   string `json:"file_id,omitempty"`
	FileName string `json:"file_name,omitempty"`
}

type TelegramVoice struct {
	FileID string `json:"file_id,omitempty"`
}

type TelegramAnimation struct {
	FileID   string `json:"file_id,omitempty"`
	FileName string `json:"file_name,omitempty"`
}

type TelegramChatMemberUpdated struct {
	Chat TelegramChat `json:"chat"`
}

type MaxWebhookUpdate struct {
	Message *MaxWebhookMessage `json:"message,omitempty"`
}

type MaxWebhookBody struct {
	Text string `json:"text,omitempty"`
}

type MaxWebhookMessage struct {
	Text   string          `json:"text,omitempty"`
	Body   *MaxWebhookBody `json:"body,omitempty"`
	Sender MaxSenderRef    `json:"sender"`
}

type MaxSenderRef struct {
	UserID    int64  `json:"user_id"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

type Invite struct {
	ID        int64
	ScopeType string
	ScopeID   string
	CodeHash  string
	ExpiresAt time.Time
	UsedAt    *time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
	SingleUse bool
	Metadata  map[string]any
}

type LinkedUser struct {
	ID                 int64
	MaxUserID          int64
	IsActive           bool
	IsBlocked          bool
	LinkedAt           time.Time
	LastDeliveryStatus string
	LastDeliveryAt     *time.Time
}

type Route struct {
	ID                 int64
	TelegramGroupID    int64
	TelegramChatID     int64
	MaxUserID          int64
	Enabled            bool
	FilterMode         RouteFilterMode
	IgnoreBotMessages  bool
	LastDeliveryStatus string
	LastDeliveryError  string
	UpdatedAt          time.Time
}

type DeliveryJob struct {
	ID                int64
	RouteID           int64
	TelegramChatID    int64
	TelegramMessageID int64
	MaxUserID         int64
	PayloadJSON       []byte
	Status            DeliveryJobStatus
	Attempts          int
	MaxAttempts       int
	AvailableAt       time.Time
	LeasedUntil       *time.Time
	LastError         string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type RetryPolicy struct {
	MaxAttempts int
}
