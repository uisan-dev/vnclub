package server

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

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
		v1.GET("/rooms/:id/tracks", s.HandleListTracks)
		v1.GET("/rooms/:id/checkpoints", s.HandleListCheckpoints)
		v1.GET("/rooms/:id/members", s.HandleListMembers)

		authed := v1.Group("")
		authed.Use(s.RequireAuth())
		{

			authed.GET("/search", s.HandleSearch)

			authed.GET("/self", s.HandleSelf)
			authed.GET("/self/rooms", s.HandleMyRooms)
			authed.POST("/rooms", s.HandleCreateRoom)

			authed.POST("/rooms/:id/join", s.HandleJoinRoom)
			authed.DELETE("/rooms/:id/leave", s.HandleLeaveRoom)

			authed.POST("/rooms/:id/tracks", s.HandleCreateTrack)
			authed.DELETE("/rooms/:id/tracks/:trackID", s.HandleDeleteTrack)
			authed.POST("/rooms/:id/tracks/:trackID/checkpoints", s.HandleCreateCheckpoint)
			authed.DELETE("/rooms/:id/tracks/:trackID/checkpoints", s.HandleDeleteLastCheckpoint)
			authed.PUT("/rooms/:id/tracks/:trackID/progress", s.HandleSetProgress)

			authed.GET("/rooms/:id/progress", s.HandleGetProgress)

			authed.GET("/rooms/:id/comments", s.HandleListComments)
			authed.POST("/rooms/:id/comments", s.HandleCreateComment)
			authed.DELETE("/rooms/:id/comments/:commentID", s.HandleDeleteComment)
		}
	}

	return r
}
