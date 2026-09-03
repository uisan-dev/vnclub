package store

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log"
	"os"
	"strings"
	"time"
	"vnclub/util"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(path string) (*Store, error) {
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})

	db.AutoMigrate(&Room{}, &User{}, &Session{})

	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) CreateRoom(r *Room) error {
	return s.db.Create(r).Error
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
