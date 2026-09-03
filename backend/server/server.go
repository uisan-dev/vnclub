package server

import (
	"errors"
	"log"
	"net/http"
	"vnclub/store"
	"vnclub/util"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func NewServer(dbPath string) *Server {
	st, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("could not open SQLite database: %v", err)
	}

	return &Server{
		Store:         st,
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

		authed := v1.Group("")
		authed.Use(s.RequireAuth())
		{
			authed.GET("/self", s.HandleSelf)
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
