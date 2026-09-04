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
		v1.GET("/rooms/:id", s.HandleGetRoom) // TODO: Return 404 if it is invite only and the user is not in it
		v1.GET("/rooms/:id/checkpoints", s.HandleListCheckpoints)

		authed := v1.Group("")
		authed.Use(s.RequireAuth())
		{
			authed.GET("/self", s.HandleSelf)
			authed.POST("/rooms", s.HandleCreateRoom)

			authed.POST("/rooms/:id/join", s.HandleJoinRoom)
			authed.DELETE("/rooms/:id/leave", s.HandleLeaveRoom)
			authed.PUT("/rooms/:id/progress", s.HandleSetProgress)
			authed.POST("/rooms/:id/checkpoints", s.HandleCreateCheckpoint)
			authed.DELETE("/rooms/:id/checkpoints/last", s.HandleDeleteLastCheckpoint)
		}
	}

	return r
}
