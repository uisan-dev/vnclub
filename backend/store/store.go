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

	err = db.AutoMigrate(&Room{}, &User{}, &Session{}, &Media{}, &Checkpoint{}, &RoomMember{}, &MemberProgress{}, &Track{}, &Comment{})

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
	err := s.db.Preload("User").Where("room_id = ?", roomID).Order("is_owner desc, joined_at asc").Find(&members).Error
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

func (s *Store) SetProgress(roomID, userID, trackID uint, position int) error {
	p := MemberProgress{
		RoomID:    roomID,
		UserID:    userID,
		TrackID:   trackID,
		Position:  position,
		UpdatedAt: time.Now(),
	}

	return s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "room_id"}, {Name: "user_id"}, {Name: "track_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"position", "updated_at"}),
	}).Create(&p).Error
}

func (s *Store) MemberTrackProgress(roomID, userID uint) (map[uint]int, error) {
	var rows []MemberProgress
	err := s.db.Where("room_id = ? AND user_id = ?", roomID, userID).Find(&rows).Error
	if err != nil {
		return nil, err
	}

	out := make(map[uint]int, len(rows))
	for _, r := range rows {
		out[r.TrackID] = r.Position
	}
	return out, nil
}

// Wow
func (s *Store) AllProgress(roomID uint) (map[uint]map[uint]int, error) {
	var rows []MemberProgress
	if err := s.db.Where("room_id = ?", roomID).Find(&rows).Error; err != nil {
		return nil, err
	}

	out := make(map[uint]map[uint]int)
	for _, r := range rows {
		if out[r.UserID] == nil {
			out[r.UserID] = make(map[uint]int)
		}
		out[r.UserID][r.TrackID] = r.Position
	}

	return out, nil
}

func (s *Store) GenerateCheckpoints(roomID, trackID uint, count int, label string) error {
	if count <= 0 {
		return nil
	}

	if label == "" {
		label = "Part"
	}

	cps := make([]Checkpoint, 0, count)
	for i := 1; i <= count; i++ {
		cps = append(cps, Checkpoint{
			RoomID:   roomID,
			TrackID:  trackID,
			Position: i,
			Label:    fmt.Sprintf("%s %d", label, i),
		})
	}
	return s.db.CreateInBatches(cps, 256).Error
}

func (s *Store) CreateCheckpoint(roomID, trackID uint, label string) (*Checkpoint, error) {
	var maxPos int
	err := s.db.Model(&Checkpoint{}).Where("track_id = ?", trackID).Select("COALESCE(MAX(position), 0)").Scan(&maxPos).Error
	if err != nil {
		return nil, err
	}

	cp := &Checkpoint{
		RoomID:   roomID,
		TrackID:  trackID,
		Position: maxPos + 1,
		Label:    strings.TrimSpace(label),
	}
	if err := s.db.Create(&cp).Error; err != nil {
		return nil, err
	}

	return cp, nil
}

func (s *Store) ListCheckpoints(roomID uint) ([]Checkpoint, error) {
	var cps []Checkpoint
	err := s.db.Preload("Track").Where("room_id = ?", roomID).Order("track_id asc, position asc").Find(&cps).Error
	return cps, err
}

func (s *Store) ListTrackCheckpoints(trackID uint) ([]Checkpoint, error) {
	var cps []Checkpoint
	err := s.db.Where("track_id = ?", trackID).Order("position asc").Find(&cps).Error
	return cps, err
}

func (s *Store) CountCheckpoints(roomID uint) (int, error) {
	var n int64
	err := s.db.Model(&Checkpoint{}).Where("room_id = ?", roomID).Count(&n).Error
	return int(n), err
}

func (s *Store) CountTrackCheckpoints(trackID uint) (int, error) {
	var n int64
	err := s.db.Model(&Checkpoint{}).Where("track_id = ?", trackID).Count(&n).Error
	return int(n), err
}

func (s *Store) DeleteLastCheckpoint(trackID uint) (bool, error) {
	var cp Checkpoint
	err := s.db.Where("track_id = ?", trackID).Order("position desc").First(&cp).Error
	if err != nil {
		return false, err
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&cp).Error; err != nil {
			return err
		}

		if err := tx.Where("track_id = ? AND position >= ?", trackID, cp.Position).
			Delete(&Comment{}).Error; err != nil {
			return err
		}
		return tx.Model(&MemberProgress{}).
			Where("track_id = ? AND position >= ?", trackID, cp.Position).
			Update("position", cp.Position-1).Error
	})

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

func (s *Store) CreateComment(c *Comment) error {
	return s.db.Create(c).Error
}

func (s *Store) DeleteComment(commentID, roomID, userID uint, isOwner bool) (bool, error) {
	q := s.db.Where("id = ? AND room_id = ?", commentID, roomID)
	if !isOwner {
		q = q.Where("user_id = ?", userID)
	}

	res := q.Delete(&Comment{})
	return res.RowsAffected > 0, res.Error
}

func (s *Store) GetComment(id uint) (*Comment, error) {
	var c Comment
	if err := s.db.Preload("User").Where("id = ?", id).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) CreateTrack(roomID uint, label string) (*Track, error) {
	var maxSort int
	err := s.db.Model(&Track{}).Where("room_id = ?", roomID).Select("COALESCE(MAX(sort), 0)").Scan(&maxSort).Error
	if err != nil {
		return nil, err
	}

	track := &Track{
		RoomID: roomID,
		Slug:   slugify(label),
		Label:  strings.TrimSpace(label),
		Sort:   maxSort + 1,
	}

	if err := s.db.Create(&track).Error; err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, ErrTrackExists
		}
		return nil, err
	}

	return track, nil
}

func (s *Store) ListTracks(roomID uint) ([]Track, error) {
	var tracks []Track
	err := s.db.Where("room_id = ?", roomID).Find(&tracks).Error
	return tracks, err
}

func (s *Store) GetTrack(roomID, trackID uint) (*Track, error) {
	var t Track
	err := s.db.Where("id = ? AND room_id = ?", trackID, roomID).First(&t).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) DeleteTrack(roomID, trackID uint) (bool, error) {
	var removed bool

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("track_id = ?", trackID).Delete(&Comment{}).Error; err != nil {
			return err
		}
		if err := tx.Where("track_id = ?", trackID).Delete(&MemberProgress{}).Error; err != nil {
			return err
		}
		if err := tx.Where("track_id = ?", trackID).Delete(&Checkpoint{}).Error; err != nil {
			return err
		}

		res := tx.Where("id = ? AND room_id = ?", trackID, roomID).Delete(&Track{})
		if res.Error != nil {
			return res.Error
		}
		removed = res.RowsAffected > 0
		return nil
	})

	return removed, err
}

// VisibleComments returns every comment the reader has unlocked, across
// all tracks. The LEFT JOIN plus COALESCE is the whole spoiler system:
// a member with no progress row for a track is treated as position 0, so
// nothing needs backfilling when someone joins or a track is added.
func (s *Store) VisibleComments(roomID, userID uint) ([]Comment, error) {
	var comments []Comment

	err := s.db.Preload("User").Preload("Track").
		Joins(`LEFT JOIN member_progress mp
		         ON mp.room_id  = comments.room_id
		        AND mp.track_id = comments.track_id
		        AND mp.user_id  = ?`, userID).
		Where("comments.room_id = ? AND comments.position <= COALESCE(mp.position, 0)", roomID).
		Order("comments.track_id asc, comments.position asc, comments.created_at asc").
		Find(&comments).Error

	return comments, err
}

// VisibleTrackComments is the same gate, narrowed to one track.
func (s *Store) VisibleTrackComments(roomID, userID, trackID uint) ([]Comment, error) {
	var comments []Comment

	err := s.db.Preload("User").Preload("Track").
		Joins(`LEFT JOIN member_progress mp
		         ON mp.room_id  = comments.room_id
		        AND mp.track_id = comments.track_id
		        AND mp.user_id  = ?`, userID).
		Where("comments.room_id = ? AND comments.track_id = ? AND comments.position <= COALESCE(mp.position, 0)",
			roomID, trackID).
		Order("comments.position asc, comments.created_at asc").
		Find(&comments).Error

	return comments, err
}

func (s *Store) CountComments(roomID uint) (int, error) {
	var n int64
	err := s.db.Model(&Comment{}).Where("room_id = ?", roomID).Count(&n).Error
	return int(n), err
}

// HiddenCommentCount is everything in the room minus what this reader
// can see. Two simple queries beat one clever one here, and a bare count
// leaks nothing about content.
func (s *Store) HiddenCommentCount(roomID, userID uint) (int, error) {
	total, err := s.CountComments(roomID)
	if err != nil {
		return 0, err
	}

	visible, err := s.VisibleComments(roomID, userID)
	if err != nil {
		return 0, err
	}

	return total - len(visible), nil
}

// HiddenPerTrack breaks the hidden count down by track, so the UI can
// say "8 more on Tomoyo's route" without revealing any of them.
func (s *Store) HiddenPerTrack(roomID, userID uint) (map[uint]int, error) {
	type row struct {
		TrackID uint
		Total   int
	}

	var totals []row
	err := s.db.Model(&Comment{}).
		Select("track_id, COUNT(*) AS total").
		Where("room_id = ?", roomID).
		Group("track_id").Scan(&totals).Error
	if err != nil {
		return nil, err
	}

	visible, err := s.VisibleComments(roomID, userID)
	if err != nil {
		return nil, err
	}

	seen := make(map[uint]int, len(totals))
	for _, c := range visible {
		seen[c.TrackID]++
	}

	out := make(map[uint]int, len(totals))
	for _, t := range totals {
		if n := t.Total - seen[t.TrackID]; n > 0 {
			out[t.TrackID] = n
		}
	}
	return out, nil
}

func (s *Store) CreateBranchingTrack(roomID, parentID uint, branchAt int, label string) (*Track, error) {
	if parentID != 0 {
		parent, err := s.GetTrack(roomID, parentID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrParentNotFound
		}
		if err != nil {
			return nil, err
		}

		total, err := s.CountTrackCheckpoints(parent.ID)
		if err != nil {
			return nil, err
		}
		if branchAt > total {
			return nil, ErrBranchPastParent
		}
	} else {
		branchAt = 0
	}

	var maxSort int
	err := s.db.Model(&Track{}).Where("room_id = ?", roomID).
		Select("COALESCE(MAX(sort), 0)").Scan(&maxSort).Error
	if err != nil {
		return nil, err
	}

	track := &Track{
		RoomID:   roomID,
		ParentID: parentID,
		BranchAt: branchAt,
		Slug:     slugify(label),
		Label:    strings.TrimSpace(label),
		Sort:     maxSort + 1,
	}

	if err := s.db.Create(track).Error; err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, ErrTrackExists
		}
		return nil, err
	}

	return track, nil
}

func (s *Store) CountChildTracks(parentID uint) (int, error) {
	var n int64
	err := s.db.Model(&Track{}).Where("parent_id = ?", parentID).Count(&n).Error
	return int(n), err
}

func (s *Store) DeleteLeafTrack(roomID, trackID uint) (bool, error) {
	children, err := s.CountChildTracks(trackID)
	if err != nil {
		return false, err
	}
	if children > 0 {
		return false, ErrTrackHasChildren
	}
	return s.DeleteTrack(roomID, trackID)
}

// TrackTree assembles the full picture for one member in three queries:
// the tracks, their checkpoint counts, and the member's progress.
func (s *Store) TrackTree(roomID, userID uint) ([]TrackNode, error) {
	tracks, err := s.ListTracks(roomID)
	if err != nil {
		return nil, err
	}
	if len(tracks) == 0 {
		return nil, nil
	}

	type countRow struct {
		TrackID uint
		Total   int
	}
	var counts []countRow
	err = s.db.Model(&Checkpoint{}).
		Select("track_id, COUNT(*) AS total").
		Where("room_id = ?", roomID).
		Group("track_id").Scan(&counts).Error
	if err != nil {
		return nil, err
	}

	cpCount := make(map[uint]int, len(counts))
	for _, c := range counts {
		cpCount[c.TrackID] = c.Total
	}

	progress := map[uint]int{}
	if userID != 0 {
		progress, err = s.MemberTrackProgress(roomID, userID)
		if err != nil {
			return nil, err
		}
	}

	open := Availability(tracks, progress)
	depth := Depths(tracks)

	nodes := make([]TrackNode, 0, len(tracks))
	for i := range tracks {
		t := tracks[i]
		nodes = append(nodes, TrackNode{
			Track:       t,
			Depth:       depth[t.ID],
			Available:   open[t.ID],
			Checkpoints: cpCount[t.ID],
			Progress:    progress[t.ID],
		})
	}
	return nodes, nil
}

// RoomsForUser returns the rooms this user is a member of, with member
// counts, in one query.
//
// Two joins do different jobs: the INNER JOIN on "mine" filters to rooms
// the user belongs to, and the LEFT JOIN on "everyone" counts all their
// members. Using one join for both would count only the caller.
func (s *Store) RoomsForUser(userID uint, limit int) ([]RoomWithCount, error) {
	var rooms []RoomWithCount

	err := s.db.Model(&Room{}).
		Select("rooms.*, COUNT(everyone.user_id) AS member_count").
		Joins("JOIN room_members mine ON mine.room_id = rooms.id AND mine.user_id = ?", userID).
		Joins("LEFT JOIN room_members everyone ON everyone.room_id = rooms.id").
		Group("rooms.id").
		Order("rooms.created_at DESC").
		Limit(limit).
		Preload("Owner").Preload("Media").
		Find(&rooms).Error

	return rooms, err
}

// RoomsOwnedBy returns rooms this user created, including invite-only
// ones, which ListRooms deliberately hides.
func (s *Store) RoomsOwnedBy(userID uint, limit int) ([]RoomWithCount, error) {
	var rooms []RoomWithCount

	err := s.db.Model(&Room{}).
		Select("rooms.*, COUNT(room_members.user_id) AS member_count").
		Joins("LEFT JOIN room_members ON room_members.room_id = rooms.id").
		Where("rooms.owner_id = ?", userID).
		Group("rooms.id").
		Order("rooms.created_at DESC").
		Limit(limit).
		Preload("Owner").Preload("Media").
		Find(&rooms).Error

	return rooms, err
}

// IsTrackAvailable answers the question for one track, used to reject
// progress on a route the member has not unlocked.
func (s *Store) IsTrackAvailable(roomID, userID, trackID uint) (bool, error) {
	tracks, err := s.ListTracks(roomID)
	if err != nil {
		return false, err
	}

	progress, err := s.MemberTrackProgress(roomID, userID)
	if err != nil {
		return false, err
	}

	return Availability(tracks, progress)[trackID], nil
}
