package types

import "time"

type VNResponse struct {
	VN                 string `json:"vn"`
	HasLanguageSupport bool   `json:"has_language_support"` // Check user's language from DB, and then check if the VN has a version/patch for it
}

type VN struct {
	ID               string      `json:"id" binding:"required"`
	Title            VNTitle     `json:"title"`
	Aliases          []string    `json:"aliases"`
	OriginalLanguage string      `json:"original_language"`
	DevStatus        int         `json:"devstatus"`
	ReleaseDate      time.Time   `json:"released"`
	Languages        []string    `json:"languages"`
	Platforms        []string    `json:"platforms"`
	Length           int         `json:"length"`
	Description      string      `json:"description"`
	URL              string      `json:"url"` // VNDB entry URL
	Rating           string      `json:"rating"`
	VoteCount        int         `json:"vote_count"`
	Developers       []Developer `json:"developers"`
	IsNSFW           bool        `json:"is_nsfw"`
	FetchedAt        time.Time
}

func (vn VN) HasLanguageSupport(lang string) bool {
	for _, l := range vn.Languages {
		if l == lang {
			return true
		}
	}
	return false
}

type VNCoverImage struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	ThumbURL  string `json:"thumbnail_url"`
	IsSexual  bool   `json:"is_sexual"`
	IsViolent bool   `json:"is_violent"`
}

type RelatedVN struct {
	ID        string  `json:"id" binding:"required"`
	Title     VNTitle `json:"title"`
	ThumbURL  string  `json:"thumbnail_url"`
	URL       string  `json:"url"` // VNDB entry URL
	Rating    int     `json:"rating"`
	VoteCount int     `json:"vote_count"`
}

type VNTitle struct {
	Original string `json:"original"`
	Latin    string `json:"latin"`
}

type Developer struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Aliases []string `json:"aliases"`
	Type    string   `json:"type"`
}
