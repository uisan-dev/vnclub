package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"vnclub/server"

	"github.com/joho/godotenv"
)

func main() {

	godotenv.Load()

	srv := server.NewServer("vnclub.db")

	httpSrv := &http.Server{
		Addr:    ":8080",
		Handler: srv.Router(),
	}

	out, err := srv.VNDB.MediaByID("v7")
	defer srv.VNDB.Close()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%+v\n", out)

	go func() {
		log.Println("Listening on port 8080")
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listening and serving: %v", err)
		}
	}()

	go func() {
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			log.Println("Purging expired sessions")
			n, err := srv.Store.PurgeExpiredSessions()
			if err != nil {
				log.Printf("purging expired sessions: %v", err)
				continue
			}
			if n > 0 {
				log.Printf("Purged %d expired sessions", n)
			}
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down server")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpSrv.Shutdown(ctx); err != nil {
		log.Printf("shutting down server: %v", err)
	}

	log.Printf("Succesfully shut down")
}
