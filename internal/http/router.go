// Package api wires the HTTP routes: /metrics, /healthz, the
// Gravatar-compatible /slack/{slackOrgId}/profiles/{id} and
// /slack/{slackOrgId}/avatar/{id} endpoints, and the webcomponent.js
// static asset.
package api

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-contrib/location/v2"
	"github.com/gin-contrib/sessions"
	gormsessions "github.com/gin-contrib/sessions/gorm"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"gorm.io/gorm"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"

	"github.com/halkeye/omnitar/internal/config"
	"github.com/halkeye/omnitar/internal/models"
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

		ll := logger.WithFields(logrus.Fields{
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

func generateRandomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

var (
	providers = map[models.OAuthProvider]func(*Deps, *gin.Context) *oauth2.Config{
		models.Slack:     getSlackOauth,
		models.Atlassian: getAtlassianOauth,
	}
)

func getSlackOauth(deps *Deps, c *gin.Context) *oauth2.Config {
	callbackURL := location.Get(c)
	callbackURL.Path = "/auth/slack/callback"

	return &oauth2.Config{
		ClientID:     deps.Config.SlackClientID(),
		ClientSecret: deps.Config.SlackClientSecret(),
		RedirectURL:  callbackURL.String(),
		Endpoint: oauth2.Endpoint{
			AuthURL:   "https://slack.com/oauth/v2/authorize",
			TokenURL:  "https://slack.com/api/oauth.v2.access",
			AuthStyle: oauth2.AuthStyleInParams,
		},
		Scopes: []string{"users.profile:read", "users:read", "users:read.email"},
	}
}

func getAtlassianOauth(deps *Deps, c *gin.Context) *oauth2.Config {
	callbackURL := location.Get(c)
	callbackURL.Path = "/auth/atlassian/callback"

	return &oauth2.Config{
		ClientID:     deps.Config.AtlassianClientID(),
		ClientSecret: deps.Config.AtlassianClientSecret(),
		RedirectURL:  callbackURL.String(),
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://auth.atlassian.com/authorize",
			TokenURL: "https://auth.atlassian.com/oauth/token",
			// https://api.atlassian.com/oauth/token/accessible-resources
		},
		Scopes: []string{"read:avatar:jira", "read:project.avatar:jira", "read:issue:jira", "read:issue-meta:jira"},
	}
}

// NewRouter builds the full route table for the service.
func NewRouter(deps *Deps) http.Handler {
	configureGinLogging(deps.Logger)

	store := gormsessions.NewStore(deps.Config.Database(), true, []byte(deps.Config.SessionKey()))
	router := gin.New()
	//   sub, _ := fs.Sub(tmplFS, "templates")
	// r.LoadHTMLFS(http.FS(sub), "**/*.tmpl")
	router.LoadHTMLGlob("templates/*.tmpl")
	router.Use(
		location.Default(),
		sessions.Sessions("mysession", store),
		gin.Recovery(),
		gin.ErrorLogger(),
		requestLogger(deps.Logger),
		recovery(deps.Logger),
		noTransform(),
	)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	router.GET("/healthz", healthzHandler)
	router.GET("/features", featuresHandler(deps))
	router.Static("/js", "./static/js")
	router.Static("/css", "./static/css")

	router.GET("/", func(c *gin.Context) {
		session := sessions.Default(c)
		baseURL := location.Get(c)
		baseURL.Path = "/"

		flashes := session.Flashes()
		session.Save()

		c.HTML(http.StatusOK, "index.tmpl", gin.H{"baseURL": baseURL, "Flashes": flashes})
	})

	router.GET("/auth", func(c *gin.Context) {
		session := sessions.Default(c)
		accountID := session.Get("account_id")
		if accountID == nil {
			deps.Logger.Debug("No account_id in session, redirecting to login")
			session.AddFlash("You must log in to view this page.")
			err := session.Save()
			if err != nil {
				deps.Logger.WithError(err).Error("Failed to save session")
			}

			c.Redirect(http.StatusFound, "/")
			return
		}

		dbAccount, err := gorm.G[models.Account](deps.Config.Database()).
			Preload("Tokens", nil).
			Where(models.Account{ID: uuid.MustParse(accountID.(string))}).
			First(c.Request.Context())
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to retrieve account: %w", err))
			return
		}

		flashes := session.Flashes()
		session.Save()

		c.HTML(http.StatusOK, "auth.tmpl", gin.H{"Account": dbAccount, "Flashes": flashes})
	})

	router.GET("/auth/:provider", func(c *gin.Context) {
		session := sessions.Default(c)
		state, err := generateRandomState()
		if err != nil {
			c.String(http.StatusInternalServerError, "Unable to generate state value")
			return
		}

		providerFunc := providers[models.OAuthProvider(c.Param("provider"))]
		if providerFunc == nil {
			c.AbortWithError(http.StatusBadRequest, errors.New("unsupported provider"))
			return
		}

		session.Set("state_"+state, time.Now().Format(time.RFC3339))
		err = session.Save()
		if err != nil {
			deps.Logger.WithError(err).Error("Failed to save session")
		}

		oauthConfig := providerFunc(deps, c)
		authURL := oauthConfig.AuthCodeURL(state)
		c.Redirect(http.StatusFound, authURL)
	})

	router.GET("/auth/:provider/callback", func(c *gin.Context) {
		session := sessions.Default(c)

		providerFunc := providers[models.OAuthProvider(c.Param("provider"))]
		if providerFunc == nil {
			c.AbortWithError(http.StatusBadRequest, errors.New("unsupported provider"))
			return
		}

		oauth2Config := providerFunc(deps, c)

		// Retrieve and verify state
		state := c.Query("state")
		deps.Logger.WithField("state", session.Get("state_"+state)).Debug("OAuth callback received")
		if val, ok := session.Get("state_" + state).(string); !ok || val == "" {
			c.AbortWithError(http.StatusBadRequest, errors.New("invalid state value"))
			return
		}
		session.Delete("state_" + state)
		err := session.Save()
		if err != nil {
			deps.Logger.WithError(err).Error("Failed to save session")
		}

		// Retrieve code
		code := c.Query("code")
		if code == "" {
			c.String(http.StatusBadRequest, "Authorization code not provided")
			return
		}

		ctx := c.Request.Context()
		// rt := MyRoundTripper{logger: deps.Logger}
		// ctx := context.WithValue(c.Request.Context(), oauth2.HTTPClient, &http.Client{Transport: rt})
		// Exchange code for access token
		token, err := oauth2Config.Exchange(ctx, code)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("Unable to exchange access token: %w", err))
			return
		}

		accountUUID := ""
		if val, ok := session.Get("account_id").(string); ok {
			accountUUID = val
		}

		deps.Logger.WithField("team", token.Extra("team")).Debug("OAuth callback received")
		dbToken := models.Token{AccountToken: token.AccessToken, ExpiresAt: token.Expiry.Unix(), RefreshToken: token.RefreshToken}
		switch models.OAuthProvider(c.Param("provider")) {
		case models.Slack:
			deps.Logger.Info("something")

			dbToken.Origin = "slack"
			dbToken.OriginID = token.Extra("team").(map[string]any)["id"].(string)
			// FIXME - check extra and all that is right

			dbToken, err = deps.Config.FindOrCreateTokenAndAccount(ctx, accountUUID, dbToken)
			if err != nil {
				c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to find or create slack token and account: %w", err))
				return
			}
		case models.Atlassian:
			deps.Logger.Info("something")
			deps.Logger.WithField("token", token).Debug("Retrieved access token")
			client := oauth2Config.Client(ctx, token)
			resp, err := client.Get("https://api.atlassian.com/oauth/token/accessible-resources")
			if err != nil {
				c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("Unable to retrieve user information: %w", err))
				return
			}
			defer resp.Body.Close()

			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("Unable to read response body: %w", err))
				return
			}

			type accessibleResources struct {
				ID   string `json:"id"`
				URL  string `json:"url"`
				Name string `json:"name"`
			}

			resources := []*accessibleResources{}
			err = json.Unmarshal(bodyBytes, &resources)
			if err != nil {
				c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("Unable to unmarshal accessible resources: %w", err))
				return
			}

			for _, ar := range resources {
				dbToken := dbToken
				dbToken.Origin = "atlassian"
				dbToken.OriginID = ar.URL
				dbToken, err = deps.Config.FindOrCreateTokenAndAccount(ctx, accountUUID, dbToken)
				if err != nil {
					c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to find or create atlassian token and account: %w", err))
					return
				}
			}
		}

		session.Set("account_id", dbToken.AccountUUID.String())
		err = session.Save()
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to save session: %w", err))
			return
		}

		deps.Logger.WithField("account_id", dbToken.AccountUUID).Debug("Saved account ID in session")
		c.Redirect(http.StatusFound, "/auth")
	})

	router.GET("/auth/logout", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Clear()
		session.AddFlash("logged out")
		err := session.Save()
		if err != nil {
			deps.Logger.WithError(err).Error("Failed to save session")
		}
		c.Redirect(http.StatusFound, "/")
	})

	if deps.Config.SlackClientID() != "" && deps.Config.SlackClientSecret() != "" {
		router.GET("/slack/auth", slackInstallHandler(deps))
	}

	router.GET("/slack/:slackOrgId/profiles/:profileIdentifier", cors(), profileHandler(deps))
	router.OPTIONS("/slack/:slackOrgId/profiles/:profileIdentifier", cors())
	router.GET("/slack/:slackOrgId/avatar/:profileIdentifier", cors(), avatarHandler(deps, deps.DefaultAvatar))
	router.OPTIONS("/slack/:slackOrgId/avatar/:profileIdentifier", cors())
	router.GET("/slack/:slackOrgId/webcomponent.js", cors(), webcomponentHandler(deps.StaticHandler))
	// router.NoRoute(gin.WrapH(deps.StaticHandler))

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

type MyRoundTripper struct {
	logger *logrus.Logger
}

func (t MyRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	var err error
	var reqBody []byte
	var respBody []byte

	if req.Body != nil {
		reqBody, err = io.ReadAll(req.Body)
		if err != nil {
			panic(err)
		}
		req.Body = io.NopCloser(bytes.NewReader(reqBody))
	}
	t.logger.WithFields(logrus.Fields{
		"req.method":        req.Method,
		"req.body":          string(reqBody),
		"req.url":           req.URL.String(),
		"req.header":        req.Header,
		"req.form":          req.Form,
		"req.postform":      req.PostForm,
		"req.multipartform": req.MultipartForm,
	}).Debug("Making request")
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return resp, err
	}
	if resp.Body != nil {
		respBody, err = io.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}
		resp.Body = io.NopCloser(bytes.NewReader(respBody))
	}
	t.logger.WithField("resp.status", resp.Status).WithField("resp.body", string(respBody)).Debug("got response")
	// Do work after the response is received

	return resp, err
}
