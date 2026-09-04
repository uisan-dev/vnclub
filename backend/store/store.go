package store

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
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

	err = db.AutoMigrate(&Room{}, &User{}, &Session{}, &Media{}, &Checkpoint{}, &RoomMember{})

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

func (s *Store) ListRooms(limit int) ([]RoomWithCount, error) {
	var rooms []RoomWithCount
	err := s.db.Model(&Room{}).
		Select("rooms.*, COUNT(room_members.user_id) AS member_count").
		Joins("LEFT JOIN room_members ON room_members.room_id = rooms.id").
		Where("rooms.invite_only = ?", false).
		Group("rooms.id").
		Order("rooms.created_at DESC").
		Limit(limit).
		Preload("Owner").Preload("Media").
		Find(&rooms).Error
	return rooms, err
}

func (s *Store) JoinRoom(roomID, userID uint, isOwner bool) error {
	m := RoomMember{RoomID: roomID, UserID: userID, IsOwner: isOwner, JoinedAt: time.Now()}
	err := s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&m).Error
	return err
}

func (s *Store) LeaveRoom(roomID, userID uint) (bool, error) {
	res := s.db.Where("room_id = ? AND user_id = ?", roomID, userID).Delete(&RoomMember{})
	return res.RowsAffected > 0, res.Error
}

func (s *Store) Members(roomID uint) ([]RoomMember, error) {
	var members []RoomMember
	err := s.db.Preload("User").Where("room_id = ?", roomID).Order("progress desc, joined_at asc").Find(&members).Error
	return members, err
}

func (s *Store) MemberCounts(roomIDs []uint) (map[uint]int, error) {
	counts := make(map[uint]int, len(roomIDs))
	if len(roomIDs) == 0 {
		return counts, nil
	}

	type countRow struct {
		RoomID uint
		Total  int
	}

	var rows []countRow
	err := s.db.Model(&RoomMember{}).Select("room_id, COUNT(user_id) AS total").Where("room_id IN ?", roomIDs).Group("room_id").Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	for _, r := range rows {
		counts[r.RoomID] = r.Total
	}
	return counts, nil
}

func (s *Store) GetMembership(roomID, userID uint) (*RoomMember, error) {
	var m RoomMember
	err := s.db.Preload("User").Where("room_id = ? AND user_id = ?", roomID, userID).First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (s *Store) SetProgress(roomID, userID uint, position int) (bool, error) {
	res := s.db.Model(&RoomMember{}).Where("room_id = ? AND user_id = ?", roomID, userID).Update("progress", position)
	return res.RowsAffected > 0, res.Error
}

func (s *Store) GenerateCheckpoints(roomID uint, count int, label string) error {
	if count <= 0 {
		return nil
	}

	cps := make([]Checkpoint, 0, count)
	for i := 1; i <= count; i++ {
		cps = append(cps, Checkpoint{
			RoomID:   roomID,
			Position: i,
			Label:    fmt.Sprintf("%s %d", label, i),
		})
	}
	return s.db.CreateInBatches(cps, 256).Error
}

func (s *Store) CreateCheckpoint(roomID uint, label string) (*Checkpoint, error) {
	var maxPos int
	err := s.db.Model(&Checkpoint{}).Where("room_id = ?", roomID).Select("COALESCE(MAX(position), 0)").Scan(&maxPos).Error
	if err != nil {
		return nil, err
	}

	cp := Checkpoint{
		RoomID:   roomID,
		Position: maxPos + 1,
		Label:    label,
	}
	if err := s.db.Create(&cp).Error; err != nil {
		return nil, err
	}
	return &cp, err
}

func (s *Store) ListCheckpoints(roomID uint) ([]Checkpoint, error) {
	var cps []Checkpoint
	err := s.db.Where("room_id = ?", roomID).Order("position asc").Find(&cps).Error
	return cps, err
}

func (s *Store) CountCheckpoints(roomID uint) (int, error) {
	var n int64
	err := s.db.Model(&Checkpoint{}).Where("room_id = ?", roomID).Count(&n).Error
	return int(n), err
}

func (s *Store) DeleteLastCheckpoint(roomID uint) (bool, error) {
	var cp Checkpoint
	err := s.db.Where("room_id = ?", roomID).Order("position desc").First(&cp).Error
	if err != nil {
		return false, err
	}

	if err := s.db.Delete(&cp).Error; err != nil {
		return false, err
	}

	err = s.db.Model(&RoomMember{}).Where("room_id = ? AND progress >= ?", roomID, cp.Position).Update("progress", cp.Position-1).Error

	return true, err
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
		if len(waste_hash) == 0 {
			log.Printf("WASTE_HASH not found in .env file, using default")
			waste_hash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
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
	return s.db.Where("token = ?", token).Delete(&Session{}).Error
}

func (s *Store) PurgeExpiredSessions() (int64, error) {
	res := s.db.Where("expires_at < ?", time.Now()).Delete(&Session{})
	return res.RowsAffected, res.Error
}
