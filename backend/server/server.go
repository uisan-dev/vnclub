package server

import (
	"errors"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"vnclub/anilist"
	"vnclub/club"
	"vnclub/store"
	"vnclub/util"
	"vnclub/vndb"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func NewServer(dbPath string) *Server {
	st, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("could not open SQLite database: %v", err)
	}

	return &Server{
		Store:         st,
		VNDB:          vndb.NewClient(),
		AniList:       anilist.NewClient(),
		SecureCookies: false, // TODO: Debug flag
	}
}

func (s *Server) HandleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "OK"})
}

func (s *Server) HandleRegister(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, util.ErrorJSON("Username must be 3-32 characters, email must be valid and password should be at least 8 characters long"))
		return
	}

	user, err := s.Store.CreateUser(req.Username, req.Email, req.Password)
	switch {
	case errors.Is(err, store.ErrUsernameTaken):
		c.JSON(http.StatusConflict, util.ErrorJSON("That username is taken"))
		return
	case errors.Is(err, store.ErrEmailTaken):
		c.JSON(http.StatusConflict, util.ErrorJSON("That email is already registered"))
		return
	case err != nil:
		c.JSON(http.StatusInternalServerError, util.ErrorJSON("Could not create account"))
		return
	}

	if err := s.startSession(c, user.ID); err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, util.ErrorJSON("Registration succesful but the server was unable to create a session"))
		return
	}

	c.JSON(http.StatusCreated, util.DataJSON(toUserResponse(user)))
}

func (s *Server) HandleLogin(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, util.ErrorJSON("Username and password required"))
		return
	}

	user, err := s.Store.Authenticate(req.Username, req.Password)
	if errors.Is(err, store.ErrInvalidCredentials) {
		c.JSON(http.StatusUnauthorized, util.ErrorJSON("Invalid username or password"))
		return
	}
	if err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, util.ErrorJSON("Could not log in"))
		return
	}

	if err := s.startSession(c, user.ID); err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, util.ErrorJSON("Could not log in"))
		return
	}

	c.JSON(http.StatusOK, toUserResponse(user))
}

func (s *Server) HandleLogout(c *gin.Context) {
	if token, err := c.Cookie(sessionCookie); err == nil {
		if err := s.Store.DeleteSession(token); err != nil {
			c.Error(err)
		}
	}
	s.clearSessionCookie(c)
	c.JSON(http.StatusOK, util.DataJSON(gin.H{"logged_in": true}))
}

func (s *Server) HandleSelf(c *gin.Context) {
	c.JSON(http.StatusOK, util.DataJSON(toUserResponse(CurrentUser(c))))
}

var vnIDPattern = regexp.MustCompile(`^v[1-9][0-9]*$`)

var formattedMap map[string]string = map[string]string{
	"vndb":    "VNDB",
	"anilist": "AniList",
}

func (s *Server) HandleCreateRoom(c *gin.Context) {
	var req createRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, util.ErrorJSON("vn_id is required, source must by VNDB or Anilist and room title needs to be 3-256 characters"))
		return
	}

	var media club.Media
	var merr error

	if req.Source == string(club.SourceVNDB) {
		if !vnIDPattern.MatchString(req.SourceID) {
			c.JSON(http.StatusBadRequest, util.ErrorJSON("Invalid format for source_id (VNDB)"))
			return
		}
		media, merr = s.VNDB.MediaByID(req.SourceID)
	} else {
		if vnIDPattern.MatchString(req.SourceID) {
			c.JSON(http.StatusBadRequest, util.ErrorJSON("Invalid format for source_id (AniList)"))
			return
		}
		media, merr = s.AniList.MediaByID(req.SourceID)
	}

	switch {
	case errors.Is(merr, vndb.ErrNotFound), errors.Is(merr, anilist.ErrNotFound):
		c.JSON(http.StatusNotFound, util.ErrorJSON("Could not find a media entry with that ID from "+formattedMap[req.Source]))
		return
	case errors.Is(merr, vndb.ErrRateLimit), errors.Is(merr, anilist.ErrRateLimit):
		c.JSON(http.StatusServiceUnavailable, util.ErrorJSON("Rate limit reached. Please try again shortly"))
		return
	case merr != nil:
		c.JSON(http.StatusInternalServerError, util.ErrorJSON("Could not fetch media entry from "+formattedMap[req.Source]))
		return
	}

	owner := CurrentUser(c)

	mediaRow, err := s.Store.UpsertMedia(media)
	if err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, util.ErrorJSON("Could not create the room"))
		return
	}

	var inviteCode string = ""

	if req.InviteOnly {
		inviteCode, err = generateSecureString(8)
		if err != nil {
			c.JSON(http.StatusInternalServerError, util.ErrorJSON("Could not generate invite code"))
			return
		}
	}

	room := &store.Room{
		Title:      req.Title,
		OwnerID:    owner.ID,
		InviteOnly: req.InviteOnly,
		InviteCode: inviteCode,
		MediaID:    mediaRow.ID,
	}

	if err := s.Store.CreateRoom(room); err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, util.ErrorJSON("Could not create the room"))
		return
	}

	room.Owner = *owner
	room.Media = *mediaRow

	s.Store.JoinRoom(room.ID, owner.ID, true)

	label := "Main"
	if mediaRow.Kind == string(club.KindAnime) {
		label = "Episodes"
	}

	track, err := s.Store.CreateTrack(room.ID, label)
	if err != nil {
		c.Error(err)
	} else if mediaRow.UnitCount > 0 {
		unit := mediaRow.UnitLabel
		if unit == "" {
			unit = "Episode"
		}
		if err := s.Store.GenerateCheckpoints(room.ID, track.ID, mediaRow.UnitCount, unit); err != nil {
			c.Error(err)
		}
	}

	if mediaRow.UnitCount > 0 {
		label := mediaRow.UnitLabel
		if label == "" {
			label = "Episode"
		}

		if err := s.Store.GenerateCheckpoints(room.ID, track.ID, mediaRow.UnitCount, label); err != nil {
			c.Error(err)
		}
	}

	c.JSON(http.StatusCreated, util.DataJSON(toOwnerRoomResponse(room, 1)))
}

func (s *Server) HandleGetRoom(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, util.ErrorJSON("Room ID must be a number"))
		return
	}

	user := CurrentUser(c)

	room, err := s.Store.GetRoomByID(uint(id))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, util.ErrorJSON("No room found with that ID"))
		return
	}
	if err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, util.ErrorJSON("Could not load room"))
		return
	}

	count, err := s.Store.MemberCounts([]uint{room.ID})
	if err != nil {
		c.Error(err)
	}

	if user != nil && user.ID == room.OwnerID {
		c.JSON(http.StatusOK, util.DataJSON(toOwnerRoomResponse(room, count[room.ID])))
		return
	}
	c.JSON(http.StatusOK, util.DataJSON(toRoomResponse(room, count[room.ID])))
}

func (s *Server) HandleJoinRoom(c *gin.Context) {
	room := s.roomFromParam(c)
	if room == nil {
		return
	}

	user := CurrentUser(c)

	if (room.InviteOnly && c.Query("room") != room.InviteCode) || (room.InviteOnly && user.ID != room.OwnerID) {
		c.JSON(http.StatusForbidden, util.ErrorJSON("This room is invite only"))
		return
	}

	if err := s.Store.JoinRoom(room.ID, user.ID, false); err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, util.ErrorJSON("Unable to add you to the room"))
		return
	}

	member, err := s.Store.GetMembership(room.ID, user.ID)

	if err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, util.ErrorJSON("Joined, but could not fetch membership data"))
		return
	}

	c.JSON(http.StatusOK, util.DataJSON(toMemberResponse(member)))
}

func (s *Server) HandleLeaveRoom(c *gin.Context) {
	room := s.roomFromParam(c)
	if room == nil {
		return
	}

	user := CurrentUser(c)

	if room.OwnerID == user.ID {
		c.JSON(http.StatusForbidden, util.ErrorJSON("The owner cannot leave their own room without transferring ownership"))
		return
	}

	removed, err := s.Store.LeaveRoom(room.ID, user.ID)
	if err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, util.ErrorJSON("Could not remove you from the room"))
		return
	}

	if !removed {
		c.JSON(http.StatusNotFound, util.ErrorJSON("You are not a member of this room"))
		return
	}

	c.JSON(http.StatusOK, util.DataJSON(gin.H{"left": true}))
}

func (s *Server) HandleListRooms(c *gin.Context) {

	limit := 50
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100 {
			c.JSON(http.StatusBadRequest, util.ErrorJSON("Limit must be between 1 and 100"))
			return
		}
		limit = n
	}

	rooms, err := s.Store.ListRooms(limit)
	if err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, util.ErrorJSON("Could not fetch rooms"))
		return
	}

	roomsResp := make([]roomResponse, 0, len(rooms))
	for _, r := range rooms {
		roomsResp = append(roomsResp, toRoomWithCountResponse(&r))
	}

	c.JSON(http.StatusOK, util.DataJSON(roomsResp))
}

func (s *Server) HandleDeleteLastCheckpoint(c *gin.Context) {
	room := s.roomFromParam(c)
	if room == nil {
		return
	}

	if room.OwnerID != CurrentUser(c).ID {
		c.JSON(http.StatusForbidden, util.ErrorJSON("Only the room owner can delete checkpoints"))
		return
	}

	track := s.trackFromParam(c, room.ID)
	if track == nil {
		return
	}

	removed, err := s.Store.DeleteLastCheckpoint(track.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, util.ErrorJSON("This track has no checkpoints"))
		return
	}
	if err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, util.ErrorJSON("Could not delete the checkpoint"))
		return
	}

	c.JSON(http.StatusOK, util.DataJSON(gin.H{"deleted": removed}))
}

func (s *Server) HandleSetProgress(c *gin.Context) {
	room := s.roomFromParam(c)
	if room == nil {
		return
	}

	user := CurrentUser(c)
	if member := s.requireMembership(c, room.ID, user.ID); member == nil {
		return
	}

	track := s.trackFromParam(c, room.ID)
	if track == nil {
		return
	}

	available, err := s.Store.IsTrackAvailable(room.ID, user.ID, track.ID)
	if err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, util.ErrorJSON("Could not check track availability"))
		return
	}
	if !available {
		c.JSON(http.StatusForbidden, util.ErrorJSON("You haven't unlocked "+track.Label+" yet"))
		return
	}

	var req setProgressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, util.ErrorJSON("Position must be zero or a positive number"))
		return
	}

	total, err := s.Store.CountTrackCheckpoints(track.ID)
	if err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, util.ErrorJSON("Could not fetch checkpoints"))
		return
	}

	// Progress past the last checkpoint would unlock comments that don't
	// exist yet, so reject rather than silently clamping.
	if req.Position > total {
		c.JSON(http.StatusBadRequest, util.ErrorJSON(
			"This track only has "+strconv.Itoa(total)+" checkpoints"))
		return
	}

	if err := s.Store.SetProgress(room.ID, user.ID, track.ID, req.Position); err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, util.ErrorJSON("Could not update progress"))
		return
	}

	nodes, err := s.Store.TrackTree(room.ID, user.ID)
	if err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, util.ErrorJSON("Progress saved, but could not reload tracks"))
		return
	}

	resp := make([]trackNodeResponse, 0, len(nodes))
	for i := range nodes {
		resp = append(resp, toTrackNodeResponse(&nodes[i]))
	}

	c.JSON(http.StatusOK, util.DataJSON(resp))
}

func (s *Server) HandleGetProgress(c *gin.Context) {
	room := s.roomFromParam(c)
	if room == nil {
		return
	}

	user := CurrentUser(c)
	if member := s.requireMembership(c, room.ID, user.ID); member == nil {
		return
	}

	nodes, err := s.Store.TrackTree(room.ID, user.ID)
	if err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, util.ErrorJSON("Could not fetch tracks"))
		return
	}

	resp := make([]trackNodeResponse, 0, len(nodes))
	for i := range nodes {
		resp = append(resp, toTrackNodeResponse(&nodes[i]))
	}

	c.JSON(http.StatusOK, util.DataJSON(resp))
}

func (s *Server) HandleListCheckpoints(c *gin.Context) {
	room := s.roomFromParam(c)
	if room == nil {
		return
	}

	cps, err := s.Store.ListCheckpoints(room.ID)
	if err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, util.ErrorJSON("Could not fetch checkpoints"))
		return
	}

	resp := make([]checkpointResponse, 0, len(cps))
	for _, c := range cps {
		resp = append(resp, toCheckpointResponse(&c))
	}

	c.JSON(http.StatusOK, util.DataJSON(resp))
}

func (s *Server) HandleCreateCheckpoint(c *gin.Context) {
	room := s.roomFromParam(c)
	if room == nil {
		return
	}

	if room.OwnerID != CurrentUser(c).ID {
		c.JSON(http.StatusForbidden, util.ErrorJSON("Only the room owner can create checkpoints"))
		return
	}

	track := s.trackFromParam(c, room.ID)
	if track == nil {
		return
	}

	var req createCheckpointRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, util.ErrorJSON("Label is required and should be 1-256 characters"))
		return
	}

	cp, err := s.Store.CreateCheckpoint(room.ID, track.ID, req.Label)
	if err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, util.ErrorJSON("Could not create checkpoint"))
		return
	}

	c.JSON(http.StatusCreated, util.DataJSON(toCheckpointResponse(cp)))
}

func (s *Server) HandleListComments(c *gin.Context) {
	room := s.roomFromParam(c)
	if room == nil {
		return
	}

	user := CurrentUser(c)
	if member := s.requireMembership(c, room.ID, user.ID); member == nil {
		return
	}

	var comments []store.Comment
	var err error

	// ?track=<id> narrows to one route; omitted means everything unlocked.
	if raw := c.Query("track"); raw != "" {
		trackID, convErr := strconv.ParseUint(raw, 10, 64)
		if convErr != nil {
			c.JSON(http.StatusBadRequest, util.ErrorJSON("Track filter must be a number"))
			return
		}
		comments, err = s.Store.VisibleTrackComments(room.ID, user.ID, uint(trackID))
	} else {
		comments, err = s.Store.VisibleComments(room.ID, user.ID)
	}

	if err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, util.ErrorJSON("Could not fetch comments"))
		return
	}

	hidden, err := s.Store.HiddenCommentCount(room.ID, user.ID)
	if err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, util.ErrorJSON("Could not count comments"))
		return
	}

	perTrack, err := s.Store.HiddenPerTrack(room.ID, user.ID)
	if err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, util.ErrorJSON("Could not count comments"))
		return
	}

	resp := commentListResponse{
		Comments:     make([]commentResponse, 0, len(comments)),
		Hidden:       hidden,
		HiddenTracks: make([]hiddenTrackCount, 0, len(perTrack)),
	}
	for i := range comments {
		resp.Comments = append(resp.Comments, toCommentResponse(&comments[i]))
	}
	for trackID, n := range perTrack {
		resp.HiddenTracks = append(resp.HiddenTracks, hiddenTrackCount{TrackID: trackID, Hidden: n})
	}

	c.JSON(http.StatusOK, util.DataJSON(resp))
}

func (s *Server) HandleCreateComment(c *gin.Context) {
	room := s.roomFromParam(c)
	if room == nil {
		return
	}

	user := CurrentUser(c)
	if member := s.requireMembership(c, room.ID, user.ID); member == nil {
		return
	}

	var req createCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, util.ErrorJSON(
			"track_id is required, body must be 1-4000 characters and position cannot be negative"))
		return
	}

	// Resolving the track against the room stops a track id from another
	// room being used to smuggle a comment in here.
	track, err := s.Store.GetTrack(room.ID, req.TrackID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusBadRequest, util.ErrorJSON("No track with that ID in this room"))
		return
	}
	if err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, util.ErrorJSON("Could not fetch track"))
		return
	}

	progress, err := s.Store.MemberTrackProgress(room.ID, user.ID)
	if err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, util.ErrorJSON("Could not fetch progress"))
		return
	}

	// You cannot write about part of a route you have not reached. This
	// is per track, so finishing one route grants nothing on another.
	if req.Position > progress[track.ID] {
		c.JSON(http.StatusForbidden, util.ErrorJSON(
			"You cannot comment past your own progress on "+track.Label))
		return
	}

	total, err := s.Store.CountTrackCheckpoints(track.ID)
	if err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, util.ErrorJSON("Could not fetch checkpoints"))
		return
	}
	if req.Position > total {
		c.JSON(http.StatusBadRequest, util.ErrorJSON("That checkpoint does not exist on this track"))
		return
	}

	comment := &store.Comment{
		RoomID:   room.ID,
		TrackID:  track.ID,
		UserID:   user.ID,
		Position: req.Position,
		Body:     strings.TrimSpace(req.Body),
	}

	if err := s.Store.CreateComment(comment); err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, util.ErrorJSON("Could not post the comment"))
		return
	}

	// Create does not backfill associations, so without these the
	// response carries an empty username and track label.
	comment.User = *user
	comment.Track = *track

	c.JSON(http.StatusCreated, util.DataJSON(toCommentResponse(comment)))
}

func (s *Server) HandleDeleteComment(c *gin.Context) {
	room := s.roomFromParam(c)
	if room == nil {
		return
	}

	commentID, err := strconv.ParseUint(c.Param("commentID"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, util.ErrorJSON("Comment ID must be a number"))
		return
	}

	user := CurrentUser(c)
	if member := s.requireMembership(c, room.ID, user.ID); member == nil {
		return
	}

	isOwner := room.OwnerID == user.ID

	deleted, err := s.Store.DeleteComment(uint(commentID), room.ID, user.ID, isOwner)
	if err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, util.ErrorJSON("Could not delete the comment"))
		return
	}

	// A comment belonging to someone else and one that does not exist
	// both land here on purpose, so ids can't be enumerated.
	if !deleted {
		c.JSON(http.StatusNotFound, util.ErrorJSON("No comment of yours with that ID in this room"))
		return
	}

	c.JSON(http.StatusOK, util.DataJSON(gin.H{"deleted": true}))
}

func (s *Server) HandleListTracks(c *gin.Context) {
	room := s.roomFromParam(c)
	if room == nil {
		return
	}

	var userID uint
	if user := CurrentUser(c); user != nil {
		userID = user.ID
	}

	nodes, err := s.Store.TrackTree(room.ID, userID)
	if err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, util.ErrorJSON("Could not fetch tracks"))
		return
	}

	resp := make([]trackNodeResponse, 0, len(nodes))
	for _, n := range nodes {
		resp = append(resp, toTrackNodeResponse(&n))
	}

	c.JSON(http.StatusOK, util.DataJSON(resp))
}

func (s *Server) HandleCreateTrack(c *gin.Context) {
	room := s.roomFromParam(c)
	if room == nil {
		return
	}

	if room.OwnerID != CurrentUser(c).ID {
		c.JSON(http.StatusForbidden, util.ErrorJSON("Only the room owner can add tracks"))
		return
	}

	var req createTrackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, util.ErrorJSON("Label must be 1-256 characters and branch_at must not be negative"))
		return
	}

	track, err := s.Store.CreateBranchingTrack(room.ID, req.ParentID, req.BranchAt, strings.TrimSpace(req.Label))

	switch {
	case errors.Is(err, store.ErrParentNotFound):
		c.JSON(http.StatusBadRequest, util.ErrorJSON("No parent track with that ID"))
		return
	case errors.Is(err, store.ErrBranchPastParent):
		c.JSON(http.StatusBadRequest, util.ErrorJSON("The branch point is past the end of the parent track"))
		return
	case errors.Is(err, store.ErrTrackExists):
		c.JSON(http.StatusConflict, util.ErrorJSON("Track already exists"))
	case err != nil:
		c.JSON(http.StatusInternalServerError, util.ErrorJSON("Could not create track"))
		return
	}

	c.JSON(http.StatusCreated, util.DataJSON(trackNodeResponse{
		ID:       track.ID,
		ParentID: track.ParentID,
		BranchAt: track.BranchAt,
		Slug:     track.Slug,
		Label:    track.Label,
		Sort:     track.Sort,
	}))
}

func (s *Server) HandleDeleteTrack(c *gin.Context) {
	room := s.roomFromParam(c)
	if room == nil {
		return
	}

	if room.OwnerID != CurrentUser(c).ID {
		c.JSON(http.StatusForbidden, util.ErrorJSON("Only the room owner can delete tracks"))
		return
	}

	track := s.trackFromParam(c, room.ID)
	if track == nil {
		return
	}

	removed, err := s.Store.DeleteLeafTrack(room.ID, track.ID)
	if errors.Is(err, store.ErrTrackHasChildren) {
		c.JSON(http.StatusConflict, util.ErrorJSON("Cannot delete a track without deleting child tracks first"))
		return
	}
	if err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, util.ErrorJSON("Could not delete track"))
		return
	}
	if !removed {
		c.JSON(http.StatusNotFound, util.ErrorJSON("No track with that ID in this room"))
		return
	}

	c.JSON(http.StatusOK, util.DataJSON(gin.H{"deleted": true}))
}

func (s *Server) startSession(c *gin.Context, userID uint) error {
	session, err := s.Store.CreateSession(userID)
	if err != nil {
		return err
	}

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     sessionCookie,
		Value:    session.Token,
		Path:     "/",
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		Secure:   s.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (s *Server) clearSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) roomFromParam(c *gin.Context) *store.Room {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, util.ErrorJSON("Room ID must be a number"))
		return nil
	}

	room, err := s.Store.GetRoomByID(uint(id))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, util.ErrorJSON("No room found with that ID"))
		return nil
	}
	if err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, util.ErrorJSON("Could not fetch room"))
		return nil
	}

	return room
}

func (s *Server) trackFromParam(c *gin.Context, roomID uint) *store.Track {
	id, err := strconv.ParseUint(c.Param("trackID"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, util.ErrorJSON("Track ID must be a number"))
		return nil
	}

	track, err := s.Store.GetTrack(roomID, uint(id))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, util.ErrorJSON("No track with that ID in this room"))
		return nil
	}
	if err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, util.ErrorJSON("Could not fetch track"))
		return nil
	}
	return track
}

func (s *Server) requireMembershipForComments(c *gin.Context, roomID, userID uint) *store.RoomMember {
	return s.requireMembership(c, roomID, userID)
}

func (s *Server) Close() {
	s.VNDB.Close()
}

func (s *Server) requireMembership(c *gin.Context, roomID, userID uint) *store.RoomMember {
	member, err := s.Store.GetMembership(roomID, userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusForbidden, util.ErrorJSON("Join the room first"))
		return nil
	}
	if err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, util.ErrorJSON("Could not fetch membership data"))
		return nil
	}
	return member
}
