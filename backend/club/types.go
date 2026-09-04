package club

import "time"

type MediaKind string
type MediaSource string

//TODO: Store VNDB Client and AniList Client in a map[MediaKind]Client map in the server struct
// type Client interface {
// 	MediaByID(id string) (Media, error)
// 	Search(term string, limit int) ([]Media, error)
// }

const (
	KindVisualNovel MediaKind = "vn"
	KindAnime       MediaKind = "anime"

	SourceVNDB    MediaSource = "vndb"
	SourceAniList MediaSource = "anilist"
)

type Media struct {
	ID       uint        `json:"id"`
	Source   MediaSource `json:"source"`
	SourceID string      `json:"source_id"`
	Kind     MediaKind   `json:"kind"`

	Title    string `json:"title"`
	AltTitle string `json:"alt_title,omitempty"`

	Description string `json:"description,omitempty"`
	CoverURL    string `json:"cover_url,omitempty"`
	ThumbURL    string `json:"thumbnail_url,omitempty"`

	Year        int    `json:"year,omitempty"`
	ReleaseText string `json:"release_text,omitempty"`
	Format      string `json:"format,omitempty"`

	UnitCount int    `json:"unit_count"`
	UnitLabel string `json:"unit_label"`

	Creators []Creator `json:"creators,omitempty"`

	Length string `json:"length,omitempty"`

	IsNSFW bool   `json:"is_nsfw"`
	URL    string `json:"url"`

	FetchedAt time.Time `json:"fetched_at"`
}

type Creator struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

type CreatorMediaRelation struct {
	CreatorID     string `json:"creator_id"`
	MediaSourceID string `json:"media_source_id"`
}

type Room struct {
	ID          uint      `json:"id"`
	Title       string    `json:"room_title"`
	MediaID     uint      `json:"media_id"`
	MemberCount int       `json:"member_count"`
	Tags        []string  `json:"tags"`
	InviteOnly  bool      `json:"invite_only"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type VN struct {
	ID                string      `json:"id" binding:"required"`
	Title             VNTitle     `json:"title"`
	Aliases           []string    `json:"aliases"`
	OriginalLanguage  string      `json:"original_language"`
	DevStatus         int         `json:"dev_status"`
	ReleaseDate       string      `json:"released"`
	Languages         []string    `json:"languages"`
	Platforms         []string    `json:"platforms"`
	Length            int         `json:"length"`
	Description       string      `json:"description"`
	URL               string      `json:"url"` // VNDB entry URL
	CoverURL          string      `json:"cover_url"`
	CoverThumbnailURL string      `json:"cover_thumbnail_url"`
	CoverIsSexual     bool        `json:"cover_is_sexual"`
	CoverIsViolent    bool        `json:"cover_is_violent"`
	Rating            float64     `json:"rating"`
	VoteCount         int         `json:"vote_count"`
	Developers        []Developer `json:"developers"`
	FetchedAt         time.Time
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
