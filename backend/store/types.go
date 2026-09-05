package store

import (
	"time"
	"vnclub/club"

	"gorm.io/gorm"
)

type Store struct {
	db *gorm.DB
}

type Media struct {
	ID       uint   `gorm:"primaryKey"`
	Source   string `gorm:"uniqueIndex:idx_source_ref;not null"`
	SourceID string `gorm:"uniqueIndex:idx_source_ref;not null"`

	Kind        string `gorm:"index;not null"`
	Title       string `gorm:"index;not null"`
	AltTitle    string
	Description string
	CoverURL    string
	ThumbURL    string
	Year        int
	ReleaseText string
	Format      string
	UnitCount   int
	UnitLabel   string
	Length      string
	IsNSFW      bool
	URL         string

	FetchedAt time.Time `gorm:"not null"`
}

type CreatorMediaRelation struct {
	CreatorID     string `gorm:"primaryKey"`
	MediaSourceID string `gorm:"primaryKey"`
}

func (m *Media) ToClub() club.Media {
	return club.Media{
		ID:       m.ID,
		Source:   club.MediaSource(m.Source),
		SourceID: m.SourceID,
		Kind:     club.MediaKind(m.Kind),

		Title:    m.Title,
		AltTitle: m.AltTitle,

		Description: m.Description,
		CoverURL:    m.CoverURL,
		ThumbURL:    m.ThumbURL,

		Year:        m.Year,
		ReleaseText: m.ReleaseText,
		Format:      m.Format,

		Length: m.Length,

		IsNSFW: m.IsNSFW,
		URL:    m.URL,

		UnitCount: m.UnitCount,
		UnitLabel: m.UnitLabel,

		FetchedAt: m.FetchedAt,
	}
}

type Room struct {
	ID         uint   `gorm:"primaryKey"`
	Title      string `gorm:"not null"`
	OwnerID    uint   `gorm:"index;not null"`
	Owner      User   `gorm:"foreignKey:OwnerID"`
	MediaID    uint   `gorm:"index;not null"`
	Media      Media  `gorm:"foreignKey:MediaID"`
	InviteOnly bool
	InviteCode string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type RoomConfig struct {
	RoomID                          uint `gorm:"primaryKey;autoIncrement:false;unique"`
	CanComment                      bool
	CanOnlyCommentOnCurrentPosition bool
}

type RoomWithCount struct {
	Room
	MemberCount int
}

type User struct {
	ID           uint   `gorm:"primaryKey"`
	Username     string `gorm:"uniqueIndex;not null"`
	Email        string `gorm:"uniqueIndex;not null"`
	PasswordHash string `gorm:"not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Session struct {
	Token     string    `gorm:"primaryKey"`
	UserID    uint      `gorm:"index;not null"`
	ExpiresAt time.Time `gorm:"index;not null"`
	CreatedAt time.Time
}

type Track struct {
	ID       uint   `gorm:"primaryKey"`
	ParentID uint   `gorm:"index"`
	RoomID   uint   `gorm:"uniqueIndex:idx_track_room_slug;not null"`
	BranchAt int    `gorm:"not null;default:0"`
	Slug     string `gorm:"uniqueIndex:idx_track_room_slug;not null"`
	Sort     int    `gorm:"not null;default:0"`
	Label    string `gorm:"not null"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

type Checkpoint struct {
	ID        uint   `gorm:"primaryKey"`
	RoomID    uint   `gorm:"index;not null"`
	TrackID   uint   `gorm:"uniqueIndex:idx_checkpoint_track_position;not null"`
	Track     Track  `gorm:"foreignKey:TrackID"`
	Position  int    `gorm:"uniqueIndex:idx_checkpoint_track_position;not null"`
	Label     string `gorm:"not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type RoomMember struct {
	RoomID uint `gorm:"primaryKey;autoIncrement:false"`
	UserID uint `gorm:"primaryKey;autoIncrement:false"`
	User   User `gorm:"foreignKey:UserID"`
	Room   Room `gorm:"foreignKey:RoomID"`

	IsOwner   bool `gorm:"not null;default:0"`
	JoinedAt  time.Time
	UpdatedAt time.Time
}

type MemberProgress struct {
	RoomID  uint `gorm:"primaryKey;autoIncrement:false"`
	UserID  uint `gorm:"primaryKey;autoIncrement:false"`
	TrackID uint `gorm:"primaryKey;autoIncrement:false"`

	Position  int `gorm:"not null;default:0"`
	UpdatedAt time.Time
}

func (MemberProgress) TableName() string { return "member_progress" }

type Comment struct {
	ID        uint   `gorm:"primaryKey"`
	RoomID    uint   `gorm:"index;not null"`
	TrackID   uint   `gorm:"index:idx_comment_track_position;not null"`
	Track     Track  `gorm:"foreignKey:TrackID"`
	UserID    uint   `gorm:"index;not null"`
	User      User   `gorm:"foreignKey:UserID"`
	Position  int    `gorm:"index:idx_comment_track_position;not null"`
	Body      string `gorm:"not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TrackNode is a track plus everything derived from its place in the
// tree. Depth and Available are computed, never stored.
type TrackNode struct {
	Track       Track
	Depth       int
	Available   bool
	Checkpoints int
	Progress    int
}

// Availability reports which tracks a member may start.
//
// A root track is always available. A branch opens once the member's
// progress on its parent reaches BranchAt, and only if the parent was
// itself available. That second condition is what makes prerequisites
// transitive: After Story requires Nagisa requires Common, without any
// of that being written down twice.
//
// Results are memoised, so a deep tree costs one pass rather than one
// walk per node. Marking a node false before recursing means a malformed
// cycle terminates instead of overflowing the stack.
func Availability(tracks []Track, progress map[uint]int) map[uint]bool {
	byID := make(map[uint]*Track, len(tracks))
	for i := range tracks {
		byID[tracks[i].ID] = &tracks[i]
	}

	open := make(map[uint]bool, len(tracks))
	visiting := make(map[uint]bool, len(tracks))

	var resolve func(id uint) bool
	resolve = func(id uint) bool {
		if v, done := open[id]; done {
			return v
		}
		if visiting[id] {
			return false // cycle
		}
		visiting[id] = true
		defer delete(visiting, id)

		t := byID[id]
		if t == nil {
			open[id] = false
			return false
		}

		if t.ParentID == 0 {
			open[id] = true
			return true
		}

		ok := resolve(t.ParentID) && progress[t.ParentID] >= t.BranchAt
		open[id] = ok
		return ok
	}

	for i := range tracks {
		resolve(tracks[i].ID)
	}
	return open
}

// Depths returns how far each track sits from its root.
func Depths(tracks []Track) map[uint]int {
	byID := make(map[uint]*Track, len(tracks))
	for i := range tracks {
		byID[tracks[i].ID] = &tracks[i]
	}

	depth := make(map[uint]int, len(tracks))
	visiting := make(map[uint]bool, len(tracks))

	var resolve func(id uint) int
	resolve = func(id uint) int {
		if d, done := depth[id]; done {
			return d
		}
		if visiting[id] {
			return 0
		}
		visiting[id] = true
		defer delete(visiting, id)

		t := byID[id]
		if t == nil || t.ParentID == 0 {
			depth[id] = 0
			return 0
		}

		d := resolve(t.ParentID) + 1
		depth[id] = d
		return d
	}

	for i := range tracks {
		resolve(tracks[i].ID)
	}
	return depth
}
