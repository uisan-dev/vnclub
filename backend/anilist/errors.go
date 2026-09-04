package anilist

import "errors"

var (
	ErrNotFound  = errors.New("no anime with that ID")
	ErrRateLimit = errors.New("AniList rate limit reached")
)
