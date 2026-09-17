// Package api wires the HTTP routes: /metrics, /healthz, the
// Gravatar-compatible /slack/{slackOrgId}/profiles/{id} and
// /slack/{slackOrgId}/avatar/{id} endpoints, and the webcomponent.js
// static asset.
package api

import (
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

	providers map[models.OAuthProvider]func(*gin.Context) *oauth2.Config
}

type RouterOptions func(r *Deps)

func WithLogger(logger *logrus.Logger) RouterOptions {
	return func(r *Deps) {
		r.Logger = logger
	}
}

func WithDefaultAvatar(avatar []byte) RouterOptions {
	return func(r *Deps) {
		r.DefaultAvatar = avatar
	}
}

func WithStaticHandler(handler http.Handler) RouterOptions {
	return func(r *Deps) {
		r.StaticHandler = handler
	}
}

func WithConfig(cfg config.Config) RouterOptions {
	return func(r *Deps) {
		r.Config = cfg
	}
}

func WithSlackAPIURL(url string) RouterOptions {
	return func(r *Deps) {
		r.SlackAPIURL = url
	}
}

func New(opts ...RouterOptions) *Deps {
	d := &Deps{}
	d.DefaultAvatar = DefaultAvatarSVG()
	d.providers = map[models.OAuthProvider]func(*gin.Context) *oauth2.Config{
		models.Slack:     d.getSlackOauth,
		models.Atlassian: d.getAtlassianOauth,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

func (router *Deps) webcomponentHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.URL.Path = "/webcomponent.js"
		router.StaticHandler.ServeHTTP(c.Writer, c.Request)
	}
}

func (router *Deps) cors() gin.HandlerFunc {
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
	return gin.CustomRecovery(func(c *gin.Context, err any) {
		if router.Logger != nil {
			router.Logger.WithField("error", err).Error("recovered from panic")
		}
		c.AbortWithStatus(http.StatusInternalServerError)
	})
}

func (router *Deps) configureGinLogging() {
	if router.Logger == nil {
		return
	}

	gin.DebugPrintFunc = router.Logger.Debugf
	gin.DebugPrintRouteFunc = func(method, path, handler string, handlers int) {
		router.Logger.WithFields(logrus.Fields{
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

func (router *Deps) getSlackOauth(c *gin.Context) *oauth2.Config {
	callbackURL := location.Get(c)
	callbackURL.Path = "/auth/slack/callback"

	return &oauth2.Config{
		ClientID:     router.Config.SlackClientID(),
		ClientSecret: router.Config.SlackClientSecret(),
		RedirectURL:  callbackURL.String(),
		Endpoint: oauth2.Endpoint{
			AuthURL:   "https://slack.com/oauth/v2/authorize",
			TokenURL:  "https://slack.com/api/oauth.v2.access",
			AuthStyle: oauth2.AuthStyleInParams,
		},
		Scopes: []string{"users.profile:read", "users:read", "users:read.email"},
	}
}

func (router *Deps) getAtlassianOauth(c *gin.Context) *oauth2.Config {
	callbackURL := location.Get(c)
	callbackURL.Path = "/auth/atlassian/callback"

	return &oauth2.Config{
		ClientID:     router.Config.AtlassianClientID(),
		ClientSecret: router.Config.AtlassianClientSecret(),
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
func (router *Deps) NewHTTPHandler() http.Handler {
	router.configureGinLogging()

	store := gormsessions.NewStore(router.Config.Database(), true, []byte(router.Config.SessionKey()))
	ginrouter := gin.New()
	//   sub, _ := fs.Sub(tmplFS, "templates")
	// r.LoadHTMLFS(http.FS(sub), "**/*.tmpl")
	ginrouter.LoadHTMLGlob("templates/*.tmpl")
	ginrouter.Use(
		location.Default(),
		sessions.Sessions("mysession", store),
		router.requestLogger(),
		router.recovery(),
		router.noTransform(),
	)
	ginrouter.GET("/metrics", gin.WrapH(promhttp.Handler()))
	ginrouter.GET("/healthz", router.healthzHandler)
	ginrouter.Static("/js", "./static/js")
	ginrouter.Static("/css", "./static/css")

	ginrouter.GET("/", func(c *gin.Context) {
		session := sessions.Default(c)
		baseURL := location.Get(c)
		baseURL.Path = "/"

		flashes := session.Flashes()
		session.Save()

		c.HTML(http.StatusOK, "index.tmpl", gin.H{"baseURL": baseURL, "Flashes": flashes})
	})

	ginrouter.GET("/auth", func(c *gin.Context) {
		session := sessions.Default(c)
		accountID := session.Get("account_id")
		if accountID == nil {
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

	ginrouter.GET("/auth/:provider", func(c *gin.Context) {
		session := sessions.Default(c)
		state, err := generateRandomState()
		if err != nil {
			c.String(http.StatusInternalServerError, "Unable to generate state value")
			return
		}

		providerFunc := router.providers[models.OAuthProvider(c.Param("provider"))]
		if providerFunc == nil {
			c.AbortWithError(http.StatusBadRequest, errors.New("unsupported provider"))
			return
		}

		session.Set("state_"+state, time.Now().Format(time.RFC3339))
		err = session.Save()
		if err != nil {
			router.Logger.WithError(err).Error("Failed to save session")
		}

		oauthConfig := providerFunc(c)
		authURL := oauthConfig.AuthCodeURL(state)
		c.Redirect(http.StatusFound, authURL)
	})

	ginrouter.GET("/auth/:provider/callback", func(c *gin.Context) {
		session := sessions.Default(c)

		providerFunc := router.providers[models.OAuthProvider(c.Param("provider"))]
		if providerFunc == nil {
			c.AbortWithError(http.StatusBadRequest, errors.New("unsupported provider"))
			return
		}

		oauth2Config := providerFunc(c)

		// Retrieve and verify state
		state := c.Query("state")
		router.Logger.WithField("state", session.Get("state_"+state)).Debug("OAuth callback received")
		if val, ok := session.Get("state_" + state).(string); !ok || val == "" {
			c.AbortWithError(http.StatusBadRequest, errors.New("invalid state value"))
			return
		}
		session.Delete("state_" + state)
		err := session.Save()
		if err != nil {
			router.Logger.WithError(err).Error("Failed to save session")
		}

		// Retrieve code
		code := c.Query("code")
		if code == "" {
			c.String(http.StatusBadRequest, "Authorization code not provided")
			return
		}

		ctx := c.Request.Context()
		// rt := MyRoundTripper{logger: router.Logger}
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

		router.Logger.WithField("team", token.Extra("team")).Debug("OAuth callback received")
		dbToken := models.Token{AccountToken: token.AccessToken, ExpiresAt: token.Expiry.Unix(), RefreshToken: token.RefreshToken}
		switch models.OAuthProvider(c.Param("provider")) {
		case models.Slack:
			router.Logger.Info("something")

			dbToken.Origin = "slack"
			dbToken.OriginID = token.Extra("team").(map[string]any)["id"].(string)
			// FIXME - check extra and all that is right

			dbToken, err = router.Config.FindOrCreateTokenAndAccount(ctx, accountUUID, dbToken)
			if err != nil {
				c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to find or create slack token and account: %w", err))
				return
			}
		case models.Atlassian:
			router.Logger.Info("something")
			router.Logger.WithField("token", token).Debug("Retrieved access token")
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
				dbResourceToken := dbToken
				dbResourceToken.Origin = "atlassian"
				dbResourceToken.OriginID = ar.URL
				dbToken, err = router.Config.FindOrCreateTokenAndAccount(ctx, accountUUID, dbResourceToken)
				if err != nil {
					c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to find or create atlassian token and account: %w", err))
					return
				}
			}
		}

		if dbToken.AccountUUID == nil {
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("no account associated with token after OAuth callback"))
			return
		}

		session.Set("account_id", dbToken.AccountUUID.String())
		err = session.Save()
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to save session: %w", err))
			return
		}

		router.Logger.WithField("account_id", dbToken.AccountUUID).Debug("Saved account ID in session")
		c.Redirect(http.StatusFound, "/auth")
	})

	ginrouter.GET("/auth/logout", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Clear()
		session.AddFlash("logged out")
		err := session.Save()
		if err != nil {
			router.Logger.WithError(err).Error("Failed to save session")
		}
		c.Redirect(http.StatusFound, "/")
	})

	if router.Config.SlackClientID() != "" && router.Config.SlackClientSecret() != "" {
		ginrouter.GET("/slack/auth", router.slackInstallHandler())
	}

	ginrouter.GET("/slack/:slackOrgId/profiles/:profileIdentifier", router.cors(), router.profileHandler())
	ginrouter.OPTIONS("/slack/:slackOrgId/profiles/:profileIdentifier", router.cors())
	ginrouter.GET("/slack/:slackOrgId/avatar/:profileIdentifier", router.cors(), router.avatarHandler())
	ginrouter.OPTIONS("/slack/:slackOrgId/avatar/:profileIdentifier", router.cors())
	ginrouter.GET("/slack/:slackOrgId/webcomponent.js", router.cors(), router.webcomponentHandler())

	ginrouter.GET("/account/:accountUUID/profiles/:profileIdentifier", router.cors(), router.profileHandler())
	ginrouter.GET("/account/:accountUUID/webcomponent.js", router.cors(), router.webcomponentHandler())
	ginrouter.NoRoute(gin.WrapH(router.StaticHandler))

	return ginrouter
}
