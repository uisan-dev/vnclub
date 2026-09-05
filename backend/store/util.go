package store

import (
	"regexp"
	"strings"
)

var validStringRegex = regexp.MustCompile(`^\w+$`)
var slugStrip = regexp.MustCompile(`[^a-z0-9]+`)

func isValidString(in string) bool {
	return validStringRegex.MatchString(in)
}

func slugify(label string) string {
	s := slugStrip.ReplaceAllString(strings.ToLower(strings.TrimSpace(label)), "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "track"
	}
	if len(s) > 48 {
		s = s[:48]
	}
	return s
}

// TrackAvailability reports which tracks a member can start. A root is
// always available; a branch opens once the member's progress on its
// parent reaches BranchAt, and only if the parent itself was available.
func availability(tracks []Track, progress map[uint]int) map[uint]bool {
	byID := make(map[uint]*Track, len(tracks))
	for i := range tracks {
		byID[tracks[i].ID] = &tracks[i]
	}

	open := make(map[uint]bool, len(tracks))
	var resolve func(id uint) bool

	resolve = func(id uint) bool {
		if v, done := open[id]; done {
			return v
		}
		open[id] = false // guards against a malformed cycle

		t := byID[id]
		if t == nil {
			return false
		}
		if t.ParentID == 0 {
			open[id] = true
			return true
		}

		ok := resolve(t.ParentID) && progress[t.ParentID] >= t.BranchAt
		open[id] = ok
		return ok
	}

	for i := range tracks {
		resolve(tracks[i].ID)
	}
	return open
}
