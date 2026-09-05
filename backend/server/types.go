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
	InviteCode  string        `json:"invite_code,omitempty"`
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

func toOwnerRoomResponse(r *store.Room, members int) roomResponse {
	return roomResponse{
		ID:          r.ID,
		Title:       r.Title,
		Media:       toMediaResponse(&r.Media),
		OwnerName:   r.Owner.Username,
		MemberCount: members,
		InviteOnly:  r.InviteOnly,
		InviteCode:  r.InviteCode,
		CreatedAt:   r.CreatedAt,
	}
}

func toRoomWithCountResponse(r *store.RoomWithCount) roomResponse {
	return roomResponse{
		ID:          r.ID,
		Title:       r.Title,
		Media:       toMediaResponse(&r.Media),
		OwnerName:   r.Owner.Username,
		MemberCount: r.MemberCount,
		InviteOnly:  r.InviteOnly,
		CreatedAt:   r.CreatedAt,
	}
}

type memberResponse struct {
	UserID   uint      `json:"user_id"`
	Username string    `json:"username"`
	Progress int       `json:"progress"`
	IsOwner  bool      `json:"is_owner"`
	JoinedAt time.Time `json:"joined_at"`
}

type checkpointResponse struct {
	ID       uint   `json:"id"`
	TrackID  uint   `json:"track_id"`
	Position int    `json:"position"`
	Label    string `json:"label"`
}

type createCheckpointRequest struct {
	Label string `json:"label" binding:"required,min=1,max=256"`
}

// Position is not "required": the validator reads that as "not the zero
// value", and 0 legitimately means "not started".
type setProgressRequest struct {
	Position int `json:"position" binding:"min=0"`
}

func toMemberResponse(m *store.RoomMember) memberResponse {
	return memberResponse{
		UserID:   m.UserID,
		Username: m.User.Username,
		IsOwner:  m.IsOwner,
		JoinedAt: m.JoinedAt,
	}
}

func toCheckpointResponse(c *store.Checkpoint) checkpointResponse {
	return checkpointResponse{ID: c.ID, Position: c.Position, Label: c.Label}
}

type commentResponse struct {
	ID         uint      `json:"id"`
	TrackID    uint      `json:"track_id"`
	TrackLabel string    `json:"track_label"`
	UserID     uint      `json:"user_id"`
	Username   string    `json:"username"`
	Position   int       `json:"position"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type hiddenTrackCount struct {
	TrackID uint `json:"track_id"`
	Hidden  int  `json:"hidden"`
}

type commentListResponse struct {
	Comments     []commentResponse  `json:"comments"`
	Hidden       int                `json:"hidden"`
	HiddenTracks []hiddenTrackCount `json:"hidden_by_track"`
}

type createCommentRequest struct {
	TrackID  uint   `json:"track_id" binding:"required"`
	Position int    `json:"position" binding:"min=0"`
	Body     string `json:"body" binding:"required,min=1,max=4000"`
}

func toCommentResponse(c *store.Comment) commentResponse {
	return commentResponse{
		ID:         c.ID,
		TrackID:    c.TrackID,
		TrackLabel: c.Track.Label,
		UserID:     c.UserID,
		Username:   c.User.Username,
		Position:   c.Position,
		Body:       c.Body,
		CreatedAt:  c.CreatedAt,
		UpdatedAt:  c.UpdatedAt,
	}
}

// trackNodeResponse is flat on purpose. Nesting it server-side would
// force the client to walk the tree to update one node; a flat list with
// parent_id is trivial to index, and building the tree in JS is a
// three-line reduce.
type trackNodeResponse struct {
	ID          uint   `json:"id"`
	ParentID    uint   `json:"parent_id"`
	BranchAt    int    `json:"branch_at"`
	Depth       int    `json:"depth"`
	Slug        string `json:"slug"`
	Label       string `json:"label"`
	Sort        int    `json:"sort"`
	Checkpoints int    `json:"checkpoints"`
	Progress    int    `json:"progress"`
	Available   bool   `json:"available"`

	// Unstarted is true when a track is open but untouched: exactly the
	// set the UI should offer as "what next?".
	Unstarted bool `json:"unstarted"`
}

type createTrackRequest struct {
	Label string `json:"label" binding:"required,min=1,max=256"`

	// ParentID omitted or 0 creates a root track.
	ParentID uint `json:"parent_id"`

	// BranchAt is the position on the parent where this branch opens.
	// Not "required": 0 is meaningful, it means "from the start".
	BranchAt int `json:"branch_at" binding:"min=0"`
}

func toTrackNodeResponse(n *store.TrackNode) trackNodeResponse {
	return trackNodeResponse{
		ID:          n.Track.ID,
		ParentID:    n.Track.ParentID,
		BranchAt:    n.Track.BranchAt,
		Depth:       n.Depth,
		Slug:        n.Track.Slug,
		Label:       n.Track.Label,
		Sort:        n.Track.Sort,
		Checkpoints: n.Checkpoints,
		Progress:    n.Progress,
		Available:   n.Available,
		Unstarted:   n.Available && n.Progress == 0,
	}
}

type trackResponse struct {
	ID          uint   `json:"id"`
	Slug        string `json:"slug"`
	Label       string `json:"label"`
	Sort        int    `json:"sort"`
	Checkpoints int    `json:"checkpoints"`
	Progress    int    `json:"progress"`
}
