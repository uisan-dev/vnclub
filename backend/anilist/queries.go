package anilist

import _ "embed"

//go:embed queries/media.graphql
var mediaQuery string

//go:embed queries/search.graphql
var searchQuery string
