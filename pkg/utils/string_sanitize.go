package utils

import "strings"

func SanitizeTagKey(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return "custom"
	}
	var b strings.Builder
	lastUnderscore := false
	for _, r := range text {
		keep := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || r == '_'
		if keep {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteRune('_')
			lastUnderscore = true
		}
	}
	result := strings.Trim(b.String(), "_")
	if result == "" {
		return "custom"
	}
	return result
}

func SanitizeMACID(mac string) string {
	mac = strings.ReplaceAll(strings.ToLower(mac), ":", "_")
	var b strings.Builder
	for _, r := range mac {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
