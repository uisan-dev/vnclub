package vndb

import "errors"

var (
	ErrNotFound   = errors.New("no visual novels with that ID")
	ErrRateLimit  = errors.New("vndb rate limit reached")
	ErrNotValidID = errors.New("not a valid AniList ID")
)
