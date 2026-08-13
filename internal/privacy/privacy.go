package privacy

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/ai-coding-remote/admin-platform/internal/model"
)

const Redacted = "[redacted]"

var allowedFields = map[string]struct{}{
	"attempt": {}, "available_mb": {}, "bytes": {}, "cached_transcripts": {},
	"connection_id": {}, "console_entries": {}, "count": {}, "delay_ms": {},
	"dropped_count": {}, "dropped_events": {}, "duration_ms": {}, "item_type": {},
	"live_characters": {}, "live_items": {}, "message_type": {}, "outcome": {},
	"phase": {}, "pid": {}, "project_count": {}, "projects": {},
	"prompt_characters": {}, "reason": {}, "relay_mb": {}, "relay_messages": {},
	"role": {}, "status": {}, "thread_count": {}, "threads": {},
	"transcript_messages": {}, "transport": {},
}

var (
	urlPattern  = regexp.MustCompile(`(?i)\b(?:https?|wss?)://\S+`)
	pathPattern = regexp.MustCompile(`(?:^|\s)(?:/Users/|/home/|/private/|/var/|/tmp/)[^\s]+`)
)

func SanitizeEvent(event *model.Event) {
	event.Message = sanitizeText(event.Message, 240)
	var raw map[string]any
	if json.Unmarshal(event.Fields, &raw) != nil {
		event.Fields = json.RawMessage(`{}`)
		return
	}
	clean := make(map[string]any, len(raw))
	for key, value := range raw {
		key = strings.ToLower(strings.TrimSpace(key))
		if _, ok := allowedFields[key]; !ok {
			continue
		}
		if sanitized, ok := sanitizeValue(value); ok {
			clean[key] = sanitized
		}
	}
	encoded, _ := json.Marshal(clean)
	event.Fields = encoded
}

func sanitizeValue(value any) (any, bool) {
	switch typed := value.(type) {
	case string:
		return sanitizeText(typed, 160), true
	case float64, bool, nil:
		return typed, true
	default:
		return nil, false
	}
}

func sanitizeText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if urlPattern.MatchString(value) || pathPattern.MatchString(value) {
		return Redacted
	}
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit]) + "..."
}
