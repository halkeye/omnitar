package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/slack-go/slack"

	"gorm.io/gorm"

	"github.com/halkeye/omnitar/internal/directory"
	"github.com/halkeye/omnitar/internal/http/gintemplrenderer"
	"github.com/halkeye/omnitar/internal/models"
	"github.com/halkeye/omnitar/internal/sessions"
	"github.com/halkeye/omnitar/internal/templates"
)

func (router *Deps) healthzHandler(c *gin.Context) {
	c.String(http.StatusOK, "ok")
}

// profileHandler serves Gravatar-style profile JSON for a known
// profileIdentifier (MD5 or SHA256 hash of a normalized email).
func (router *Deps) profileHandler(c *gin.Context) {
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

// avatarHandler serves a Gravatar-style avatar lookup: a known
// profileIdentifier 302s to the cached Slack avatar URL; an unknown one
// honors the Gravatar ?d= convention (d=404 -> 404, anything else -> the
// bundled default silhouette).
func (router *Deps) avatarHandler(c *gin.Context) {
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

func (router *Deps) handlerIndexPage(c *gin.Context) {
	r := gintemplrenderer.New(c.Request.Context(), http.StatusOK, templates.Index())
	c.Render(http.StatusOK, r)
}

func (router *Deps) handlerMyAccountPage(c *gin.Context) {
	var err error
	dbAccount := getTyped[models.Account](c, "account")

	tokens, err := gorm.G[*models.Token](router.Config.Database()).Where(models.Token{AccountUUID: new(dbAccount.ID)}).Find(c.Request.Context())
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to load tokens: %w", err))
		return
	}

	r := gintemplrenderer.New(c.Request.Context(), http.StatusOK, templates.AccountMyPage(&dbAccount, tokens))
	c.Render(http.StatusOK, r)
}

func (router *Deps) handlerAuthSigninPage(c *gin.Context) {
	r := gintemplrenderer.New(c.Request.Context(), http.StatusOK, templates.AuthSigninPage())
	c.Render(http.StatusOK, r)
}

func (router *Deps) handlerAuthRegisterPage(c *gin.Context) {
	session := sessions.Default(c)
	sessions.Set(session, "account_id", "83334a23-7a1d-463b-8830-4d0b9400703a")
	sessions.MustSave(session)
	c.Redirect(http.StatusFound, "/account/my")
	return

	var dbAccount models.Account
	err := gorm.G[models.Account](router.Config.Database()).Create(c.Request.Context(), &dbAccount)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to create account: %w", err))
		return
	}

	// session := sessions.Default(c)
	// sessions.Set(session, "account_id", dbAccount.ID.String())
	// sessions.MustSave(session)
	// c.Redirect(http.StatusFound, "/account/my")

	r := gintemplrenderer.New(c.Request.Context(), http.StatusOK, templates.AuthRegisterPage(&dbAccount))
	c.Render(http.StatusOK, r)
}

func (router *Deps) handlerAuthLogout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.AddFlash("logged out")
	sessions.MustSave(session)
	c.Redirect(http.StatusFound, "/")
}

func (router *Deps) webcomponentHandler(c *gin.Context) {
	c.Request.URL.Path = "/webcomponent.js"
	router.StaticHandler.ServeHTTP(c.Writer, c.Request)
}

func (router *Deps) profileHandler2(c *gin.Context) {
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

func (router *Deps) deleteTokenHandler(c *gin.Context) {
	session := sessions.Default(c)

	dbAccount := getTyped[models.Account](c, "account")

	tokenID := c.Param("token")
	if tokenID == "" {
		router.Logger.Debug("No token ID provided")
		c.Status(http.StatusNotFound)
		return
	}

	_, err := gorm.G[models.Token](router.Config.Database()).Where(models.Token{ID: uuid.MustParse(tokenID), AccountUUID: new(dbAccount.ID)}).Delete(c.Request.Context())
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to delete token: %w", err))
		return
	}

	session.AddFlash("Token deleted successfully.")
	c.Status(http.StatusNoContent)
}

func (router *Deps) handlerProvider(c *gin.Context) {
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

	sessions.Set(session, "state_"+state, time.Now().Format(time.RFC3339))
	sessions.MustSave(session)

	oauthConfig := providerFunc(c)
	authURL := oauthConfig.AuthCodeURL(state)
	c.Redirect(http.StatusFound, authURL)
}

func (router *Deps) handlerProviderCallback(c *gin.Context) {
	session := sessions.Default(c)

	providerFunc := router.providers[models.OAuthProvider(c.Param("provider"))]
	if providerFunc == nil {
		c.AbortWithError(http.StatusBadRequest, errors.New("unsupported provider"))
		return
	}

	oauth2Config := providerFunc(c)

	// Retrieve and verify state
	state := c.Query("state")
	stateVal := sessions.Get[string](session, "state_"+state)
	router.Logger.WithField("state", stateVal).Debug("OAuth callback received")
	if stateVal == "" {
		c.AbortWithError(http.StatusBadRequest, errors.New("invalid state value"))
		return
	}
	session.Delete("state_" + state)
	sessions.MustSave(session)

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

	accountUUID := sessions.Get[string](session, "account_id")

	router.Logger.WithField("team", token.Extra("team")).Debug("OAuth callback received")
	dbToken := models.Token{AccountToken: token.AccessToken, ExpiresAt: token.Expiry.Unix(), RefreshToken: token.RefreshToken}
	switch models.OAuthProvider(c.Param("provider")) {
	case models.Slack:
		router.Logger.Info("something")

		for _, field := range []string{"enterprise", "team"} {
			extra, ok := token.Extra(field).(map[string]any)
			router.Logger.WithField("field", field).WithField("extra", extra).Debug("Retrieved extra field from token")
			if !ok {
				continue
			}
			originID, ok := extra["id"].(string)
			router.Logger.WithField("field", field).WithField("originID", originID).Debug("Retrieved origin ID from extra field")
			if !ok {
				continue
			}

			dbSlackToken := dbToken
			dbSlackToken.Origin = "slack"
			dbSlackToken.OriginID = originID
			dbToken, err = router.Config.FindOrCreateTokenAndAccount(ctx, accountUUID, dbSlackToken)
			if err != nil {
				c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to find or create slack token and account: %w", err))
				return
			}
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

	sessions.Set(session, "account_id", dbToken.AccountUUID.String())
	sessions.MustSave(session)

	router.Logger.WithField("account_id", dbToken.AccountUUID).Debug("Saved account ID in session")
	c.Redirect(http.StatusFound, "/auth")
}
