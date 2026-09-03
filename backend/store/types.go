package store

import (
	"time"

	"gorm.io/gorm"
)

type Store struct {
	db *gorm.DB
}

type Room struct {
	ID         uint   `gorm:"primaryKey"`
	Title      string `gorm:"not null"`
	VNID       string `gorm:"index;not null"`
	VNTitle    string
	InviteOnly bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
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
