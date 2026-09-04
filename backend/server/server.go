package server

import (
	"errors"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"vnclub/anilist"
	"vnclub/club"
	"vnclub/store"
	"vnclub/util"
	"vnclub/vndb"

	"github.com/gin-contrib/cors"
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

func (s *Server) Router() *gin.Engine {
	r := gin.Default()

	corsCfg := cors.DefaultConfig()
	corsCfg.AllowOrigins = []string{"http://localhost:5173"}
	corsCfg.AllowMethods = []string{"GET", "POST", "DELETE", "PATCH", "OPTIONS"}
	r.Use(cors.New(corsCfg))

	r.GET("/health", s.HandleHealth)

	v1 := r.Group("/api/v1")
	{
		v1.POST("/register", s.HandleRegister)
		v1.POST("/login", s.HandleLogin)
		v1.POST("/logout", s.HandleLogout)

		v1.GET("/rooms", s.HandleListRooms)
		v1.GET("/rooms/:id", s.HandleGetRoom)

		authed := v1.Group("")
		authed.Use(s.RequireAuth())
		{
			authed.GET("/self", s.HandleSelf)
			authed.POST("/rooms", s.HandleCreateRoom)
		}
	}

	return r
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
	c.JSON(http.StatusOK, util.DataJSON(true))
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

	room := &store.Room{
		Title:      req.Title,
		OwnerID:    owner.ID,
		InviteOnly: req.InviteOnly,
		MediaID:    mediaRow.ID,
	}

	if err := s.Store.CreateRoom(room); err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, util.ErrorJSON("Could not create the room"))
		return
	}

	room.Owner = *owner
	room.Media = *mediaRow
	c.JSON(http.StatusCreated, util.DataJSON(toRoomResponse(room, 1)))
}

func (s *Server) HandleGetRoom(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, util.ErrorJSON("Room ID must be a number"))
		return
	}

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

	c.JSON(http.StatusOK, util.DataJSON(toRoomResponse(room, 1)))
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
		roomsResp = append(roomsResp, toRoomResponse(&r, 1))
	}

	c.JSON(http.StatusOK, util.DataJSON(roomsResp))
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

func (s *Server) Close() {
	s.VNDB.Close()
}
