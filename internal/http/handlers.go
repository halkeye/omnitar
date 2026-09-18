package api

import (
	"fmt"
	"net/http"

	"github.com/gin-contrib/location/v2"
	"github.com/gin-contrib/sessions"
	"github.com/google/uuid"
	"github.com/halkeye/omnitar/internal/directory"
	"github.com/halkeye/omnitar/internal/models"
	"gorm.io/gorm"

	"github.com/gin-gonic/gin"
	"github.com/slack-go/slack"
)

func (router *Deps) healthzHandler(c *gin.Context) {
	c.String(http.StatusOK, "ok")
}

// profileHandler serves Gravatar-style profile JSON for a known
// profileIdentifier (MD5 or SHA256 hash of a normalized email).
func (router *Deps) profileHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		ll := router.Logger.WithField("slackOrgId", c.Param("slackOrgId"))
		source := router.Config.Source(c.Param("slackOrgId"))
		if source == nil {
			ll.Debug("unknown Slack org ID")
			c.Status(http.StatusNotFound)
			return
		}

		p, err := source.Lookup(c.Request.Context(), c.Param("profileIdentifier"))
		ll = ll.WithField("profileIdentifier", c.Param("profileIdentifier"))
		if err != nil {
			ll.Debug("error fetching profile")
			c.Error(err)
			return
		}
		if p.Email == "" {
			ll.Debug("profile not found")
			c.Status(http.StatusNotFound)
			return
		}

		c.JSON(http.StatusOK, p)
	}
}

// avatarHandler serves a Gravatar-style avatar lookup: a known
// profileIdentifier 302s to the cached Slack avatar URL; an unknown one
// honors the Gravatar ?d= convention (d=404 -> 404, anything else -> the
// bundled default silhouette).
func (router *Deps) avatarHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		source := router.Config.Source(c.Param("slackOrgId"))
		if source == nil {
			c.Status(http.StatusNotFound)
			return
		}

		p, err := source.Lookup(c.Request.Context(), c.Param("profileIdentifier"))
		if err != nil {
			c.Error(err)
			return
		}
		if p.Email == "" || p.AvatarURL == "" {
			if c.Query("d") == "404" {
				c.Status(http.StatusNotFound)
				return
			}

			c.Header("Cache-Control", "public, max-age=3600, no-transform")
			c.Data(http.StatusOK, "image/svg+xml", router.DefaultAvatar)
			return
		}

		c.Redirect(http.StatusFound, p.AvatarURL)
	}
}

func (router *Deps) slackInstallHandler() func(c *gin.Context) {
	return func(c *gin.Context) {
		_, errExists := c.GetQuery("error")
		if errExists {
			c.String(http.StatusOK, "error installing app")
			return
		}

		code, codeExists := c.GetQuery("code")
		if !codeExists {
			c.String(http.StatusBadRequest, "missing mandatory 'code' query parameter")
			return
		}

		var opts []slack.OAuthOption
		if router.SlackAPIURL != "" {
			opts = append(opts, slack.OAuthOptionAPIURL(router.SlackAPIURL))
		}
		resp, err := slack.GetOAuthV2Response(http.DefaultClient, router.Config.SlackClientID(), router.Config.SlackClientSecret(), code, "", opts...)
		if err != nil {
			c.String(http.StatusInternalServerError, "error exchanging temporary code for access token: %s", err.Error())
			return
		}

		err = router.Config.SaveSourceConnection(c.Request.Context(), "slack", resp.Team.ID, resp.AccessToken)
		if err != nil {
			c.String(http.StatusInternalServerError, "error storing slack access token: %s", err.Error())
			return
		}

		if err := router.Config.AddSource(resp.Team.ID, directory.NewSlackSource(router.Logger, resp.Team.ID, resp.AccessToken)); err != nil {
			c.String(http.StatusInternalServerError, "error starting refresher for slack source: %s", err.Error())
			return
		}

		c.Redirect(http.StatusFound, "/")
	}
}

func (router *Deps) handlerIndexPage() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)

		baseURL := location.Get(c)
		baseURL.Path = "/"

		flashes := session.Flashes()
		session.Save()

		c.HTML(http.StatusOK, "index.tmpl", gin.H{"baseURL": baseURL, "Flashes": flashes})
	}
}

func (router *Deps) handlerAuthPage() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)

		baseURL := location.Get(c)
		baseURL.Path = "/"

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

		c.HTML(http.StatusOK, "auth.tmpl", gin.H{"baseURL": baseURL, "Account": dbAccount, "Flashes": flashes})
	}
}

func (router *Deps) webcomponentHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.URL.Path = "/webcomponent.js"
		router.StaticHandler.ServeHTTP(c.Writer, c.Request)
	}
}

func (router *Deps) profileHandler2() gin.HandlerFunc {
	return func(c *gin.Context) {
		account := getTyped[models.Account](c, "account")
		token := account.GetTokenForSource("slack")
		if token == nil {
			router.Logger.WithField("accountId", account.ID).Debug("no slack token for account")
			c.Status(http.StatusNotFound)
			return
		}

		source := router.Config.Source(token.OriginID)
		if source == nil {
			router.Logger.WithField("slackOrgId", token.OriginID).Debug("unknown Slack org ID")
			c.Status(http.StatusNotFound)
			return
		}
		ll := router.Logger.WithField("slackOrgId", token.OriginID)

		p, err := source.Lookup(c.Request.Context(), c.Param("profileIdentifier"))
		ll = ll.WithField("profileIdentifier", c.Param("profileIdentifier"))
		if err != nil {
			ll.Debug("error fetching profile")
			c.Error(err)
			return
		}
		if p.Email == "" {
			ll.Debug("profile not found")
			c.Status(http.StatusNotFound)
			return
		}

		c.JSON(http.StatusOK, p)
	}
}

func (router *Deps) deleteTokenHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		// FIXME - middlewareSessionAccount
		accountID, ok := session.Get("account_id").(string)
		if !ok || accountID == "" {
			router.Logger.Debug("No account_id in session, redirecting to login")
			session.AddFlash("You must log in to view this page.")
			err := session.Save()
			if err != nil {
				router.Logger.WithError(err).Error("Failed to save session")
			}

			c.Redirect(http.StatusFound, "/")
			return
		}

		tokenID := c.Param("token")
		if tokenID == "" {
			router.Logger.Debug("No token ID provided")
			c.Status(http.StatusNotFound)
			return
		}

		_, err := gorm.G[models.Token](router.Config.Database()).Where(models.Token{ID: uuid.MustParse(tokenID), AccountUUID: new(uuid.MustParse(accountID))}).Delete(c.Request.Context())
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to delete token: %w", err))
			return
		}

		session.AddFlash("Token deleted successfully.")
		c.Status(http.StatusNoContent)
	}
}
