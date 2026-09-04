package vndb

import (
	"regexp"
	"strconv"
	"strings"
)

func producerType(t string) string {
	switch t {
	case "co":
		return "Company"
	case "in":
		return "Individual"
	case "ng":
		return "Amateur Group"
	default:
		return t
	}
}

func devStatus(d int) string {
	switch d {
	case 0:
		return "finished"
	case 1:
		return "releasing"
	case 2:
		return "cancelled"
	default:
		return "unknown"
	}
}

// "2004-06-25", "2004-06", "2004", or "TBA".
func yearFrom(released string) int {
	if len(released) < 4 {
		return 0
	}
	n, err := strconv.Atoi(released[:4])
	if err != nil {
		return 0
	}
	return n
}

var bbTag = regexp.MustCompile(`\[/?[a-zA-Z]+(=[^\]]*)?\]`)

// stripBBCode removes VNDB's markup. Spoiler tags get their contents
// removed entirely, not just the markers, since this is a spoiler-aware app.
func stripBBCode(s string) string {
	s = spoilerBlock.ReplaceAllString(s, "[spoiler removed]")
	return strings.TrimSpace(bbTag.ReplaceAllString(s, ""))
}
