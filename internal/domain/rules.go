package domain

import "fmt"

func DedupeKey(routeID, chatID, messageID int64) string {
	return fmt.Sprintf("%d:%d:%d", routeID, chatID, messageID)
}

func IsMentionMessage(msg *TelegramMessage) bool {
	if msg == nil {
		return false
	}
	for _, e := range msg.Entities {
		if e.Type == "mention" {
			return true
		}
	}
	return false
}

func IsTextMessage(msg *TelegramMessage) bool {
	if msg == nil {
		return false
	}
	return msg.Text != ""
}
