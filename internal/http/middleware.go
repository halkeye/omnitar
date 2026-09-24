package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/location/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/halkeye/omnitar/internal/models"
	"github.com/halkeye/omnitar/internal/sessions"
)

func (router *Deps) cors(c *gin.Context) {
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Methods", "GET, OPTIONS")
	c.Header("Access-Control-Allow-Headers", "Content-Type")
	if c.Request.Method == http.MethodOptions {
		c.Status(http.StatusNoContent)
		c.Abort()
		return
	}
}

func (router *Deps) accountMiddleware(c *gin.Context) {
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

func (router *Deps) middlewareSessionUser(c *gin.Context) {

	session := sessions.Default(c)
	accountID := sessions.Get[string](session, "account_id")

	if accountID == "" {
		router.Logger.Debug("No account_id in session, redirecting to login")
		session.AddFlash("You must log in to view this page.")
		err := session.Save()
		if err != nil {
			router.Logger.WithError(err).Error("Failed to save session")
		}

		c.Redirect(http.StatusFound, "/")
		return
	}

	dbAccount, err := gorm.G[models.Account](router.Config.Database()).
		Preload("Tokens", nil).
		Where(models.Account{ID: uuid.MustParse(accountID)}).
		First(c.Request.Context())

	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to retrieve account: %w", err))
		return
	}

	c.Set("account", dbAccount)
}

func (router *Deps) middlewareWebauthn(c *gin.Context) {
	baseURL := location.Get(c)
	baseURL.Path = ""

	config := &webauthn.Config{
		RPDisplayName: "omnitar",
		RPID:          strings.Split(baseURL.Host, ":")[0],
		RPOrigins:     []string{baseURL.String()},
	}

	w, err := webauthn.New(config)
	if err != nil {
		router.Logger.WithError(err).Debug("FIXME - does this need manual logging?")
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.Set("webauthn", w)
}
