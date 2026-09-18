package api

import (
	"crypto/rand"
	"encoding/base64"

	"github.com/gin-gonic/gin"
)

func generateRandomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func getTyped[T any](c *gin.Context, key any) (res T) {
	if val, ok := c.Get(key); ok && val != nil {
		res, _ = val.(T)
	}
	return
}
