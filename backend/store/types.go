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

type Checkpoint struct {
	ID        uint   `gorm:"primaryKey"`
	RoomID    uint   `gorm:"uniqueIndex:idx_room_position;not null"`
	Position  int    `gorm:"uniqueIndex:idx_room_position;not null"`
	Label     string `gorm:"not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type RoomMember struct {
	RoomID   uint `gorm:"primaryKey;autoIncrement:false"`
	UserID   uint `gorm:"primaryKey;autoIncrement:false"`
	User     User `gorm:"foreignKey:UserID"`
	Room     Room `gorm:"foreignKey:RoomID"`
	Progress int  `gorm:"not null;default:0"`

	IsOwner   bool `gorm:"not null;default:0"`
	JoinedAt  time.Time
	UpdatedAt time.Time
}
