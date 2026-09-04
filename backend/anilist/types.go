package anilist

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
	"vnclub/club"
)

type Client struct {
	http    *http.Client
	limiter *time.Ticker
}

type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

type graphQLResponse struct {
	Data   *responseData  `json:"data"`
	Errors []graphQLError `json:"errors"`
}

type graphQLError struct {
	Message string `json:"message"`
}

type responseData struct {
	Media *apiMedia `json:"Media"`
	Page  *apiPage  `json:"page"`
}

type apiPage struct {
	Media []apiMedia `json:"media"`
}

type apiMedia struct {
	ID    int `json:"id"`
	Title struct {
		Romaji  *string `json:"romaji"`
		English *string `json:"english"`
		Native  *string `json:"native"`
	} `json:"title"`

	Format   *string `json:"format"`
	Status   *string `json:"status"`
	Episodes *int    `json:"episodes"`
	Duration *int    `json:"duration"`

	StartDate struct {
		Year *int `json:"year"`
	} `json:"startDate"`

	CoverImage struct {
		Large  *string `json:"large"`
		Medium *string `json:"medium"`
	} `json:"coverImage"`

	IsAdult     bool    `json:"isAdult"`
	SiteURL     *string `json:"siteUrl"`
	Description *string `json:"description"`
}

func (am *apiMedia) toClub() club.Media {
	m := club.Media{
		Source:    club.SourceAniList,
		SourceID:  strconv.Itoa(am.ID),
		Kind:      club.KindAnime,
		UnitLabel: "Episode",
		IsNSFW:    am.IsAdult,
		FetchedAt: time.Now(),
	}

	m.Title = firstNonEmpty(am.Title.English, am.Title.Romaji, am.Title.Native)
	if am.Title.Native != nil {
		m.Title = *am.Title.Native
	}

	if am.Format != nil {
		m.Format = *am.Format
	}

	if am.Status != nil {
		m.ReleaseText = normalizeStatus(*am.Status)
	}

	if am.Episodes != nil {
		m.UnitCount = *am.Episodes
	}

	if am.Duration != nil && am.Episodes != nil {
		m.Length = fmt.Sprint(*am.Duration * *am.Episodes)
	}

	if am.StartDate.Year != nil {
		m.Year = *am.StartDate.Year
	}

	if am.CoverImage.Large != nil {
		m.CoverURL = *am.CoverImage.Large
	}

	if am.CoverImage.Medium != nil {
		m.ThumbURL = *am.CoverImage.Medium
	}

	if am.SiteURL != nil {
		m.URL = *am.SiteURL
	} else {
		m.URL = "https://anilist.co/anime/" + m.SourceID
	}

	if am.Description != nil {
		m.Description = stripHTML(*am.Description)
	}

	return m
}
