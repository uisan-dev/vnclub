package store

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log"
	"os"
	"strings"
	"time"
	"vnclub/club"
	"vnclub/util"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

func Open(path string) (*Store, error) {
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})

	if err != nil {
		return nil, err
	}

	err = db.AutoMigrate(&Room{}, &User{}, &Session{}, &Media{})

	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) UpsertMedia(m club.Media) (*Media, error) {
	row := Media{
		Source:      string(m.Source),
		SourceID:    m.SourceID,
		Kind:        string(m.Kind),
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

	err := s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "source"}, {Name: "source_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"kind", "title", "alt_title", "description", "cover_url", "thumb_url",
			"year", "release_text", "format", "unit_count", "unit_label", "length",
			"is_nsfw", "url", "fetched_at",
		}),
	}).Create(&row).Error

	if err != nil {
		return nil, err
	}

	if row.ID == 0 {
		return s.MediaBySource(m.Source, m.SourceID)
	}

	return &row, nil
}

func (s *Store) MediaBySource(source club.MediaSource, sourceID string) (*Media, error) {
	var m Media
	err := s.db.Where("source = ? AND source_id = ?", string(source), sourceID).First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (s *Store) MediaByID(id uint) (*Media, error) {
	var m Media
	if err := s.db.First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (s *Store) FreshMedia(source club.MediaSource, sourceID string, maxAge time.Duration) (*Media, bool) {
	m, err := s.MediaBySource(source, sourceID)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false
		}
		return nil, false
	}

	if time.Since(m.FetchedAt) > maxAge {
		return nil, false
	}

	return m, true
}

func (s *Store) CreateRoom(r *Room) error {
	return s.db.Create(r).Error
}

func (s *Store) GetRoomByID(id uint) (*Room, error) {
	var room Room
	if err := s.db.Preload("Owner").Preload("Media").First(&room, id).Error; err != nil {
		return nil, err
	}
	return &room, nil
}

func (s *Store) ListRooms(limit int) ([]Room, error) {
	var rooms []Room
	err := s.db.Preload("Owner").Preload("Media").Order("created_at desc").Limit(limit).Where("invite_only = false").Find(&rooms).Error
	return rooms, err
}

func (s *Store) CreateUser(username, email, password string) (*User, error) {
	if !isValidString(username) {
		return nil, ErrInvalidCharactersInUsername
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &User{
		Username:     username,
		Email:        email,
		PasswordHash: string(hash),
	}

	if err := s.db.Create(user).Error; err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			if strings.Contains(err.Error(), "username") {
				return nil, ErrUsernameTaken
			}
			return nil, ErrEmailTaken
		}
		return nil, err
	}

	return user, nil
}

func (s *Store) Authenticate(username, password string) (*User, error) {
	var user User
	err := s.db.Where("username = ?", username).First(&user).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		waste_hash := os.Getenv("WASTE_HASH")
		if len(waste_hash) > 0 {
			log.Printf("WASTE_HASH not found in .env file, using default")
			waste_hash = "c1a00067ee481027f92e6655fc071b9976efaca59f0a0d610c9c25a689d2a656"
		}
		bcrypt.CompareHashAndPassword([]byte(waste_hash), []byte(password))
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	return &user, nil
}

func (s *Store) CreateSession(userID uint) (*Session, error) {
	lifetime, err := util.GetSessionLifetime()
	if err != nil {
		log.Printf("getting session length, using default: %v", err)
		lifetime = 30 * 24 * time.Hour
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}

	session := &Session{
		Token:     base64.RawURLEncoding.EncodeToString(raw),
		UserID:    userID,
		ExpiresAt: time.Now().Add(lifetime),
	}

	if err := s.db.Create(&session).Error; err != nil {
		return nil, err
	}

	return session, nil
}

func (s *Store) GetUserBySession(token string) (*User, error) {
	var session Session
	if err := s.db.Where("token = ?", token).First(&session).Error; err != nil {
		return nil, err
	}

	if time.Now().After(session.ExpiresAt) {
		return nil, ErrSessionExpired
	}

	var user User
	if err := s.db.First(&user, session.UserID).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *Store) DeleteSession(token string) error {
	return s.db.Where("token = ?", token).Error
}

func (s *Store) PurgeExpiredSessions() (int64, error) {
	res := s.db.Where("expires_at < ?", time.Now()).Delete(&Session{})
	return res.RowsAffected, res.Error
}
