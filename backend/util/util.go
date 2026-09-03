package util

import (
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

var sessionLifetime = time.Duration(0)

func GetSessionLifetime() (time.Duration, error) {
	if sessionLifetime != 0 {
		return sessionLifetime, nil
	}

	out, err := strconv.Atoi(os.Getenv("SESSION_LIFETIME"))
	if err != nil {
		sessionLifetime = time.Duration(0)
	}
	sessionLifetime = time.Duration(out) * 24 * time.Hour
	return sessionLifetime, err
}

func ErrorJSON(msg string) gin.H {
	return gin.H{"error": gin.H{"message": msg}}
}

func DataJSON(payload any) gin.H {
	return gin.H{"data": payload}
}
