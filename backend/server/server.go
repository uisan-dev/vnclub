package server

import (
	"log"
	"vnclub/store"

	"github.com/gin-gonic/gin"
)

type Server struct {
	Store  *store.Store
	Engine *gin.Engine
}

func NewServer(dbPath string) *Server {
	st, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("could not open SQLite database: %v", err)
	}
	eng := gin.Default()

	return &Server{
		Store:  st,
		Engine: eng,
	}
}
