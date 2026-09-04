package server

import (
	"time"
	"vnclub/anilist"
	"vnclub/store"
	"vnclub/vndb"
)

type Server struct {
	Store         *store.Store
	VNDB          *vndb.Client
	AniList       *anilist.Client
	SecureCookies bool
}

type registerRequest struct {
	Username string `json:"username" binding:"required,min=3,max=32"`
	Email    string `json:"email" binding:"required,max=254"`
	Password string `json:"password" binding:"required,min=8,max=256"`
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type userResponse struct {
	ID        uint      `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}

type createRoomRequest struct {
	Source     string `json:"source" binding:"required,oneof=vndb anilist"`
	SourceID   string `json:"source_id" binding:"required,max=16"`
	Title      string `json:"title" binding:"required,min=3,max=256"`
	InviteOnly bool   `json:"invite_only"`
}

type mediaResponse struct {
	Source   string `json:"source"`
	SourceID string `json:"source_id"`
	Kind     string `json:"kind"`

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

	Length string `json:"length,omitempty"`

	IsNSFW bool   `json:"is_nsfw"`
	URL    string `json:"url"`

	FetchedAt time.Time `json:"fetched_at"`
}

type roomResponse struct {
	ID          uint          `json:"id"`
	Title       string        `json:"title"`
	Media       mediaResponse `json:"media"`
	OwnerName   string        `json:"owner_name"`
	MemberCount int           `json:"member_count"`
	InviteOnly  bool          `json:"invite_only"`
	CreatedAt   time.Time     `json:"created_at"`
}

func toMediaResponse(m *store.Media) mediaResponse {
	return mediaResponse{
		Source:      m.Source,
		SourceID:    m.SourceID,
		Kind:        m.Kind,
		Title:       m.Title,
		AltTitle:    m.AltTitle,
		Description: m.Description,
		CoverURL:    m.CoverURL,
		ThumbURL:    m.ThumbURL,
		Year:        m.Year,
		ReleaseText: m.ReleaseText,
		Format:      m.Format,
		UnitCount:   m.UnitCount,
		UnitLabel:   m.UnitLabel,
		Length:      m.Length,
		IsNSFW:      m.IsNSFW,
		URL:         m.URL,
		FetchedAt:   m.FetchedAt,
	}
}

func toRoomResponse(r *store.Room, members int) roomResponse {
	return roomResponse{
		ID:          r.ID,
		Title:       r.Title,
		Media:       toMediaResponse(&r.Media),
		OwnerName:   r.Owner.Username,
		MemberCount: members,
		InviteOnly:  r.InviteOnly,
		CreatedAt:   r.CreatedAt,
	}
}
