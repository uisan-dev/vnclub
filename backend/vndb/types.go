package vndb

import (
	"fmt"
	"net/http"
	"regexp"
	"time"
	"vnclub/club"
)

type Client struct {
	http    *http.Client
	limiter *time.Ticker
}

func NewClient() *Client {
	return &Client{
		http:    &http.Client{Timeout: 15 * time.Second},
		limiter: time.NewTicker(time.Second),
	}
}

type vndbRequest struct {
	Filters any    `json:"filters"`
	Fields  string `json:"fields"`
	Sort    string `json:"sort,omitempty"`
	Reverse bool   `json:"reverse,omitempty"`
	Results int    `json:"results,omitempty"`
	Page    int    `json:"page,omitempty"`
}

type vndbResponse struct {
	Results []vndbEntry `json:"results"`
	More    bool        `json:"more"`
	Count   int         `json:"count,omitempty"`
}

// TODO: Remove unneeded fields
type vndbEntry struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	AltTitle         *string  `json:"alttitle"`
	Aliases          []string `json:"aliases"`
	OriginalLanguage string   `json:"olang"`

	DevStatus int     `json:"devstatus"`
	Released  *string `json:"released"`

	Languages []string `json:"languages"`
	Platforms []string `json:"platforms"`

	Length        *int `json:"length"`
	LengthMinutes *int `json:"length_minutes"`
	LengthVotes   *int `json:"length_votes"`

	Description *string  `json:"description"`
	Rating      *float64 `json:"rating"`
	VoteCount   int      `json:"votecount"`

	Image      *vndbImage      `json:"image"`
	Developers []vndbDeveloper `json:"developers"`
}

var lengthMap map[int]string = map[int]string{
	1: "Very short",
	2: "Short",
	3: "Medium",
	4: "Long",
	5: "Very long",
}

func (ve *vndbEntry) toMedia() club.Media {
	vn := club.Media{
		Title:       ve.Title,
		Source:      club.SourceVNDB,
		Kind:        club.KindVisualNovel,
		ReleaseText: devStatus(ve.DevStatus),
		SourceID:    ve.ID,
		URL:         "https://vndb.org/" + ve.ID,
		UnitCount:   0,
		UnitLabel:   "parts",
		FetchedAt:   time.Now(),
	}

	if ve.AltTitle != nil && *ve.AltTitle != "" {
		vn.AltTitle = *ve.AltTitle
	}

	if ve.Description != nil {
		vn.Description = stripBBCode(*ve.Description)
	}

	if ve.LengthMinutes != nil {
		vn.Length = fmt.Sprintf("%.0f", time.Duration(*ve.LengthMinutes).Hours())
	} else if ve.Length != nil {
		vn.Length = lengthMap[*ve.Length]
	}

	if ve.Released != nil {
		vn.Year = yearFrom(*ve.Released)
	}

	if ve.Image != nil {
		vn.CoverURL = ve.Image.URL
		vn.ThumbURL = ve.Image.Thumbnail
		vn.IsNSFW = ve.Image.Sexual != nil && ve.Image.IsSexual()
	}

	return vn
}

var spoilerBlock = regexp.MustCompile(`(?s)\[spoiler\].*?\[/spoiler\]`)

type vndbImage struct {
	ID        string   `json:"id"`
	URL       string   `json:"url"`
	Thumbnail string   `json:"thumbnail"`
	Sexual    *float64 `json:"sexual"`
	Violence  *float64 `json:"violence"`
}

func (vi *vndbImage) toClub() club.VNCoverImage {
	return club.VNCoverImage{
		ID:        vi.ID,
		URL:       vi.URL,
		ThumbURL:  vi.Thumbnail,
		IsSexual:  vi.IsSexual(),
		IsViolent: vi.IsViolent(),
	}
}

func (vi *vndbImage) IsSexual() bool {
	return vi.Sexual != nil && *vi.Sexual >= 1.0
}

func (vi *vndbImage) IsViolent() bool {
	return vi.Violence != nil && *vi.Violence >= 1.0
}

type vndbDeveloper struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	OriginalName string   `json:"original"`
	Aliases      []string `json:"aliases"`
	Type         string   `json:"type"`
}

func (vd *vndbDeveloper) toClub() club.Developer {
	return club.Developer{
		ID:      vd.ID,
		Name:    vd.Name,
		Aliases: vd.Aliases,
		Type:    vd.Type,
	}
}
