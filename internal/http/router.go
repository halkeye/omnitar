// Package api wires the HTTP routes: /metrics, /healthz, the
// Gravatar-compatible /slack/{slackOrgId}/profiles/{id} and
// /slack/{slackOrgId}/avatar/{id} endpoints, and the webcomponent.js
// static asset.
package api

import (
	"context"
	"net/http"

	"github.com/gin-contrib/location/v2"
	gormsessions "github.com/gin-contrib/sessions/gorm"
	"github.com/gin-gonic/gin"
	"github.com/m4gshm/gollections/slice"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"

	"github.com/halkeye/omnitar/internal/config"
	"github.com/halkeye/omnitar/internal/debugroundtripper"
	"github.com/halkeye/omnitar/internal/http/gintemplrenderer"
	"github.com/halkeye/omnitar/internal/models"
	"github.com/halkeye/omnitar/internal/sessions"
	"github.com/halkeye/omnitar/internal/templates"
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

// ErrorHandler captures errors and returns a consistent JSON error response
func ErrorHandler(log *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next() // Process the request first

		// Check if any errors were added to the context
		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err

			log.WithError(err).Error("http error")
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": err.Error(),
			})
		}
	}
}

// NewRouter builds the full route table for the service.
func (router *Deps) NewHTTPHandler() http.Handler {
	if router.Logger != nil {
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

	store := gormsessions.NewStore(router.Config.Database(), true, []byte(router.Config.SessionKey()))
	ginrouter := gin.New()

	ginHtmlRenderer := ginrouter.HTMLRender
	ginrouter.HTMLRender = &gintemplrenderer.HTMLTemplRenderer{FallbackHtmlRenderer: ginHtmlRenderer}

	ginrouter.Use(
		ErrorHandler(router.Logger),
		location.Default(),
		sessions.Sessions("mysession", store),
		router.requestLogger(),
		// router.recovery(),
		gin.Recovery(),
		router.noTransform(),
	)

	ginrouter.Use(func(c *gin.Context) {
		client := http.Client{
			Transport: debugroundtripper.RoundTripper{Logger: router.Logger},
		}
		c.Request = c.Request.WithContext(
			context.WithValue(c.Request.Context(), oauth2.HTTPClient, &client),
		)
	})

	ginrouter.Use(func(c *gin.Context) {
		ctx := c.Request.Context()
		session := sessions.Default(c)

		baseURL := location.Get(c)
		baseURL.Path = ""

		// Create a context variable that inherits from a parent, and sets the value "test".
		ctx = templates.SetBaseURL(ctx, baseURL.String())

		flashes := session.Flashes()
		sessions.MustSave(session)

		ctx = templates.AddFlash(ctx, slice.Convert(flashes, func(flash any) string { return flash.(string) })...)
		c.Request = c.Request.WithContext(ctx)
	})

	ginrouter.GET("/metrics", gin.WrapH(promhttp.Handler()))
	ginrouter.GET("/healthz", router.healthzHandler)
	ginrouter.Static("/js", "./static/js")
	ginrouter.Static("/css", "./static/css")

	ginrouter.GET("/", router.handlerIndexPage)

	// FIXME - move to source
	ginrouter.GET("/auth/:provider", router.handlerProvider)
	ginrouter.GET("/auth/:provider/callback", router.handlerProviderCallback)

	ginrouter.DELETE("/auth/token/:token", router.middlewareSessionUser, router.deleteTokenHandler)
	ginrouter.GET("/auth/register", router.handlerAuthRegisterPage)
	ginrouter.GET("/auth/login", router.handlerAuthSigninPage)
	ginrouter.GET("/auth/logout", router.middlewareSessionUser, router.handlerAuthLogout)

	if router.Config.SlackClientID() != "" && router.Config.SlackClientSecret() != "" {
		ginrouter.GET("/slack/auth", router.slackInstallHandler())
	}

	ginrouter.GET("/slack/:slackOrgId/profiles/:profileIdentifier", router.cors, router.profileHandler)
	ginrouter.OPTIONS("/slack/:slackOrgId/profiles/:profileIdentifier", router.cors)
	ginrouter.GET("/slack/:slackOrgId/avatar/:profileIdentifier", router.cors, router.avatarHandler)
	ginrouter.OPTIONS("/slack/:slackOrgId/avatar/:profileIdentifier", router.cors)
	ginrouter.GET("/slack/:slackOrgId/webcomponent.js", router.cors, router.webcomponentHandler)

	ginrouter.GET("/account/my", router.middlewareSessionUser, router.handlerMyAccountPage)
	ginrouter.GET("/account/:accountUUID/profiles/:profileIdentifier", router.cors, router.accountMiddleware, router.profileHandler2)
	ginrouter.GET("/account/:accountUUID/avatar/:profileIdentifier", router.cors, router.avatarHandler)
	ginrouter.GET("/account/:accountUUID/webcomponent.js", router.cors, router.webcomponentHandler)

	ginrouter.GET("/webauthn/register", router.middlewareSessionUser, router.middlewareWebauthn, router.handlerPasskeyCreateChallenge)
	ginrouter.POST("/webauthn/register", router.middlewareSessionUser, router.middlewareWebauthn, router.handlerPasskeyValidateCreateChallengeResponse)
	ginrouter.GET("/webauthn/login", router.middlewareWebauthn, router.handlerPasskeyLoginChallenge)
	ginrouter.POST("/webauthn/login", router.middlewareWebauthn, router.handlerPasskeyLoginChallengeResponse)
	ginrouter.NoRoute(gin.WrapH(router.StaticHandler))

	return ginrouter
}
