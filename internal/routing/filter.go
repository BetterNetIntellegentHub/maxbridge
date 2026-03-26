package routing

import "maxbridge/internal/domain"

func PassesRouteFilter(filter domain.RouteFilterMode, ignoreBots bool, msg *domain.TelegramMessage) bool {
	if msg == nil {
		return false
	}
	if ignoreBots && msg.From != nil && msg.From.IsBot {
		return false
	}
	switch filter {
	case domain.RouteFilterTextOnly:
		return domain.IsTextMessage(msg)
	case domain.RouteFilterMentions:
		return domain.IsMentionMessage(msg)
	default:
		return true
	}
}
