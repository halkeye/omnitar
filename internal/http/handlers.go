package api

import (
	"net/http"

	"github.com/halkeye/omnitar/internal/directory"

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
