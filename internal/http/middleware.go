package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/halkeye/omnitar/internal/models"
)

func (router *Deps) cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == http.MethodOptions {
			c.Status(http.StatusNoContent)
			c.Abort()
			return
		}
	}
}

func (router *Deps) accountMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		accountUUID, err := uuid.Parse(c.Param("accountUUID"))
		if err != nil {
			c.AbortWithError(http.StatusBadRequest, fmt.Errorf("invalid account UUID: %w", err))
			return
		}

		dbAccount, err := gorm.G[models.Account](router.Config.Database()).
			Preload("Tokens", nil).
			Where(models.Account{ID: accountUUID}).
			First(c.Request.Context())
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to retrieve account: %w", err))
			return
		}

		c.Set("account", dbAccount)
	}
}

func (router *Deps) noTransform() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-transform")
	}
}

func (router *Deps) requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()

		c.Next()

		if router.Logger == nil {
			return
		}

		ll := router.Logger.WithFields(logrus.Fields{
			"client_ip": c.ClientIP(),
			"latency":   time.Since(started).String(),
			"method":    c.Request.Method,
			"path":      c.Request.URL.Path,
			"status":    c.Writer.Status(),
		})
		if len(c.Errors) != 0 {
			for _, e := range c.Errors {
				ll.WithError(e).Error("request error")
			}
		} else {
			ll.Info("request complete")
		}
	}
}

func (router *Deps) recovery() gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(router.Logger.Writer(), func(c *gin.Context, err any) {
		if router.Logger != nil {
			router.Logger.WithField("error", err).Error("recovered from panic")
		}
		c.AbortWithStatus(http.StatusInternalServerError)
	})
}
