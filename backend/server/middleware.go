package server

import (
	"net/http"
	"vnclub/store"
	"vnclub/util"

	"github.com/gin-gonic/gin"
)

const (
	sessionCookie string = "vnclub_session"
	contextUser   string = "currentUser"
)

func (s *Server) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(sessionCookie)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, util.ErrorJSON("Not signed in"))
			return
		}

		user, err := s.Store.GetUserBySession(token)
		if err != nil {
			s.clearSessionCookie(c)
			c.AbortWithStatusJSON(http.StatusUnauthorized, util.ErrorJSON("Session expired"))
			return
		}

		c.Set(contextUser, user)
		c.Next()
	}
}

func CurrentUser(c *gin.Context) *store.User {
	v, ok := c.Get(contextUser)
	if !ok {
		return nil
	}
	user, _ := v.(*store.User)
	return user
}
