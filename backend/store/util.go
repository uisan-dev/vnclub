package store

import (
	"regexp"
)

var validStringRegex = regexp.MustCompile(`^\w+$`)

func isValidString(in string) bool {
	return validStringRegex.MatchString(in)
}
