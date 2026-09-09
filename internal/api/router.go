// Package api wires the HTTP routes: /metrics, /healthz, the
// Gravatar-compatible /slack/{slackOrgId}/profiles/{id} and
// /slack/{slackOrgId}/avatar/{id} endpoints, and the webcomponent.js
// static asset.
package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"

	"github.com/halkeye/omnitar/internal/config"
)

// Deps are the dependencies NewRouter needs to build the full route table.
type Deps struct {
	Logger *logrus.Logger
	// DefaultAvatar is served in place of an unknown/missing avatar.
	DefaultAvatar []byte
	StaticHandler http.Handler
	Config        config.Config
	// SlackAPIURL overrides the Slack API base URL used for the OAuth
	// token exchange. Empty means the real Slack API. Tests point this at
	// an httptest.Server.
	SlackAPIURL string
}

func webcomponentHandler(handler http.Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.URL.Path = "/webcomponent.js"
		handler.ServeHTTP(c.Writer, c.Request)
	}
}

func cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == http.MethodOptions {
			c.Status(http.StatusNoContent)
			c.Abort()
		}
	}
}

func noTransform() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-transform")
	}
}

func requestLogger(logger *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()
		if logger == nil {
			return
		}

		logger.WithFields(logrus.Fields{
			"client_ip": c.ClientIP(),
			"latency":   time.Since(started).String(),
			"method":    c.Request.Method,
			"path":      c.Request.URL.Path,
			"status":    c.Writer.Status(),
		}).Info("request complete")
	}
}

func recovery(logger *logrus.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, err any) {
		if logger != nil {
			logger.WithField("error", err).Error("recovered from panic")
		}
		c.AbortWithStatus(http.StatusInternalServerError)
	})
}

func configureGinLogging(logger *logrus.Logger) {
	if logger == nil {
		return
	}

	gin.DebugPrintFunc = logger.Debugf
	gin.DebugPrintRouteFunc = func(method, path, handler string, handlers int) {
		logger.WithFields(logrus.Fields{
			"handler":  handler,
			"handlers": handlers,
			"method":   method,
			"path":     path,
		}).Debug("route registered")
	}
}

// NewRouter builds the full route table for the service.
func NewRouter(deps *Deps) http.Handler {
	configureGinLogging(deps.Logger)

	router := gin.New()
	router.Use(requestLogger(deps.Logger), recovery(deps.Logger), noTransform())
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	router.GET("/healthz", healthzHandler)
	router.GET("/features", featuresHandler(deps))
	if deps.Config.SlackClientID() != "" && deps.Config.SlackClientSecret() != "" {
		router.GET("/slack/auth", slackInstallHandler(deps))
	}
	router.GET("/slack/:slackOrgId/profiles/:profileIdentifier", cors(), profileHandler(deps))
	router.OPTIONS("/slack/:slackOrgId/profiles/:profileIdentifier", cors())
	router.GET("/slack/:slackOrgId/avatar/:profileIdentifier", cors(), avatarHandler(deps, deps.DefaultAvatar))
	router.OPTIONS("/slack/:slackOrgId/avatar/:profileIdentifier", cors())
	router.GET("/slack/:slackOrgId/webcomponent.js", cors(), webcomponentHandler(deps.StaticHandler))
	router.NoRoute(gin.WrapH(deps.StaticHandler))

	return router
}

func featuresHandler(deps *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		features := gin.H{}
		features["installable"] = false
		if deps.Config.SlackClientID() != "" && deps.Config.SlackClientSecret() != "" {
			features["installable"] = "https://slack.com/oauth/v2/authorize?scope=users.profile:read,users:read,users:read.email&client_id=" + deps.Config.SlackClientID()
		}

		c.JSON(http.StatusOK, features)
	}
}
