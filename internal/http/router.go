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

	"github.com/halkeye/omnitar/internal/config"
	"github.com/halkeye/omnitar/internal/database"
	"github.com/halkeye/omnitar/internal/http/gintemplrenderer"
	"github.com/halkeye/omnitar/internal/logger"
	"github.com/halkeye/omnitar/internal/models"
	"github.com/halkeye/omnitar/internal/providers"
	"github.com/halkeye/omnitar/internal/sessions"
	"github.com/halkeye/omnitar/internal/templates"
)

// Deps are the dependencies NewRouter needs to build the full route table.
type Deps struct {
	Logger *logrus.Logger
	// defaultAvatar is served in place of an unknown/missing avatar.
	defaultAvatar []byte
	StaticHandler http.Handler
	Config        config.Config
	// SlackAPIURL overrides the Slack API base URL used for the OAuth
	// token exchange. Empty means the real Slack API. Tests point this at
	// an httptest.Server.
	SlackAPIURL string
}

type RouterOptions func(r *Deps)

func WithLogger(logger *logrus.Logger) RouterOptions {
	return func(r *Deps) {
		r.Logger = logger
	}
}

func WithDefaultAvatar(avatar []byte) RouterOptions {
	return func(r *Deps) {
		r.defaultAvatar = avatar
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

func New(opts ...RouterOptions) http.Handler {
	d := &Deps{}
	d.defaultAvatar = DefaultAvatarSVG()
	for _, opt := range opts {
		opt(d)
	}

	if d.Logger != nil {
		gin.DebugPrintFunc = d.Logger.Debugf
		gin.DebugPrintRouteFunc = func(method, path, handler string, handlers int) {
			d.Logger.WithFields(logrus.Fields{
				"handler":  handler,
				"handlers": handlers,
				"method":   method,
				"path":     path,
			}).Debug("route registered")
		}
	}

	store := gormsessions.NewStore(d.Config.Database(), true, []byte(d.Config.SessionKey()))
	ginrouter := gin.New()

	providersObj := providers.New()
	providersObj.Register(providers.Slack, models.SlackSourceOAuth2Config(d.Config.SlackClientID(), d.Config.SlackClientSecret()))
	providersObj.Register(providers.Atlassian, models.AtlassianSourceOAuth2Config(d.Config.AtlassianClientID(), d.Config.AtlassianClientSecret()))

	ginrouter.Use(func(c *gin.Context) {
		ctx := c.Request.Context()
		ctx = logger.WithValue(ctx, logrus.NewEntry(d.Logger))
		ctx = database.WithValue(ctx, d.Config.Database())
		ctx = context.WithValue(ctx, templates.IsDevKey, d.Config.IsDev())
		ctx = providers.WithValue(ctx, providersObj)
		// ctx = context.WithValue(ctx, oauth2.HTTPClient, &http.Client{Transport: debugroundtripper.RoundTripper{}})
		c.Request = c.Request.WithContext(ctx)
	})

	ginHtmlRenderer := ginrouter.HTMLRender
	ginrouter.HTMLRender = &gintemplrenderer.HTMLTemplRenderer{FallbackHtmlRenderer: ginHtmlRenderer}

	ginrouter.Use(
		location.Default(),
		sessions.Sessions("omnitar", store),
		d.requestLogger(), // FIXME
		d.errorHandler(),  // FIXME
		d.recovery(),      // FIXME
		gin.Recovery(),
		d.noTransform(), // FIXME
	)

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
	ginrouter.GET("/healthz", d.healthzHandler)

	ginrouter.GET("/", d.handlerIndexPage)

	ginrouter.GET("/auth/:provider", d.handlerProvider)
	ginrouter.GET("/auth/:provider/callback", d.handlerProviderCallback)
	ginrouter.DELETE("/auth/token/:token", d.middlewareSessionUser, d.deleteTokenHandler)
	ginrouter.GET("/auth/logout", d.middlewareSessionUser, d.handlerAuthLogout)

	ginrouter.GET("/account/my", d.middlewareSessionUser, d.handlerMyAccountPage)
	ginrouter.GET("/account/:accountUUID/profiles/:source/:email", d.cors, d.accountMiddleware, d.profileHandler)
	ginrouter.GET("/account/:accountUUID/avatar/:source/:email", d.cors, d.avatarHandler)
	ginrouter.GET("/account/:accountUUID/issues/:source/:id", d.cors, d.accountMiddleware, d.issueHandler)
	ginrouter.GET("/account/:accountUUID/webcomponent.js", d.cors, d.webcomponentHandler)

	ginrouter.NoRoute(gin.WrapH(d.StaticHandler))

	return ginrouter
}
