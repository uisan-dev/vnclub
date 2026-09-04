package anilist

import (
	"regexp"
	"strings"
)

func firstNonEmpty(candidates ...*string) string {
	for _, c := range candidates {
		if c != nil && strings.TrimSpace(*c) != "" {
			return *c
		}
	}
	return ""
}

func normalizeStatus(s string) string {
	switch s {
	case "FINISHED":
		return "finished"
	case "RELEASING":
		return "releasing"
	case "NOT_YET_RELEASED":
		return "upcoming"
	case "CANCELLED":
		return "cancelled"
	case "HIATUS":
		return "hiatus"
	default:
		return strings.ToLower(s)
	}
}

var htmlTag = regexp.MustCompile(`<[^>]*>`)

func stripHTML(s string) string {
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	s = htmlTag.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "..."
}
