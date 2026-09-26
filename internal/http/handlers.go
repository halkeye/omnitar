package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"uuid"

	"github.com/gin-gonic/gin"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/halkeye/omnitar/internal/directory"
	"github.com/halkeye/omnitar/internal/http/gintemplrenderer"
	"github.com/halkeye/omnitar/internal/models"
	"github.com/halkeye/omnitar/internal/sessions"
	"github.com/halkeye/omnitar/internal/templates"
)

func (router *Deps) healthzHandler(c *gin.Context) {
	c.String(http.StatusOK, "ok")
}

// avatarHandler serves a Gravatar-style avatar lookup: a known
// profileIdentifier 302s to the cached Slack avatar URL; an unknown one
// honors the Gravatar ?d= convention (d=404 -> 404, anything else -> the
// bundled default silhouette).
func (router *Deps) avatarHandler(c *gin.Context) {
	account := getTyped[models.Account](c, "account")
	token := account.GetTokenForSource("slack")
	if token == nil {
		router.Logger.WithField("accountId", account.ID).Debug("no slack token for account")
		c.Status(http.StatusNotFound)
		return
	}

	source := token.Source(router.Logger)
	if source == nil {
		router.Logger.WithFields(token.AsLog()).Debug("unknown Slack org ID")
		c.Status(http.StatusNotFound)
		return
	}

	p, err := source.Lookup(c.Request.Context(), c.Param("profileIdentifier"))
	if err != nil && !errors.Is(err, &directory.NotFoundError{}) {
		c.Error(err)
		return
	}

	if errors.Is(err, &directory.NotFoundError{}) {
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

func (router *Deps) handlerIndexPage(c *gin.Context) {
	r := gintemplrenderer.New(c.Request.Context(), http.StatusOK, templates.Index())
	c.Render(http.StatusOK, r)
}

func (router *Deps) handlerMyAccountPage(c *gin.Context) {
	var err error
	dbAccount := getTyped[models.Account](c, "account")

	tokens, err := gorm.G[*models.Token](router.Config.Database()).Where(models.Token{AccountUUID: dbAccount.ID}).Find(c.Request.Context())
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
	if router.Config.IsDev() {
		c.Request.URL.Path = "/webcomponent/webcomponent.ts"
	} else {
		c.Request.URL.Path = "/webcomponent.js"
	}
	router.StaticHandler.ServeHTTP(c.Writer, c.Request)
}

func (router *Deps) profileHandler(c *gin.Context) {
	account := getTyped[models.Account](c, "account")
	token := account.GetTokenForSource("slack")
	if token == nil {
		router.Logger.WithField("accountId", account.ID).Debug("no slack token for account")
		c.Status(http.StatusNotFound)
		return
	}

	source := token.Source(router.Logger)
	if source == nil {
		router.Logger.WithFields(token.AsLog()).Debug("unknown Slack org ID")
		c.Status(http.StatusNotFound)
		return
	}

	ll := router.Logger.WithFields(token.AsLog())

	email := c.Param("email")
	email = strings.TrimSpace(email)
	email = strings.ToLower(email)

	ll = ll.WithField("email", email)
	p, err := source.Lookup(c.Request.Context(), email)
	if err != nil && !errors.Is(err, &directory.NotFoundError{}) {
		ll.Debug("error fetching profile")
		c.Error(err)
		return
	}

	if errors.Is(err, &directory.NotFoundError{}) {
		ll.Debug("profile not found")
		c.Status(http.StatusNotFound)
		return
	}

	c.JSON(http.StatusOK, p)
}

func (router *Deps) issueHandler(c *gin.Context) {
	account := getTyped[models.Account](c, "account")
	token := account.GetTokenForSource("slack")
	if token == nil {
		router.Logger.WithField("accountId", account.ID).Debug("no slack token for account")
		c.Status(http.StatusNotFound)
		return
	}

	source := token.Source(router.Logger)
	if source == nil {
		router.Logger.WithFields(token.AsLog()).Debug("unknown Slack org ID")
		c.Status(http.StatusNotFound)
		return
	}

	ll := router.Logger.WithFields(token.AsLog())

	id := c.Param("id")

	ll = ll.WithField("id", id)
	p, err := source.Lookup(c.Request.Context(), id)
	if err != nil && !errors.Is(err, &directory.NotFoundError{}) {
		ll.Debug("error fetching profile")
		c.Error(err)
		return
	}

	if errors.Is(err, &directory.NotFoundError{}) {
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

	_, err := gorm.G[models.Token](router.Config.Database()).Where(models.Token{ID: uuid.MustParse(tokenID), AccountUUID: dbAccount.ID}).Delete(c.Request.Context())
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
	ctx := c.Request.Context()
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

	// Exchange code for access token
	token, err := oauth2Config.Exchange(ctx, code)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("Unable to exchange access token: %w", err))
		return
	}

	router.Logger.WithField("team", token.Extra("team")).Debug("OAuth callback received")
	dbTokens := []*models.Token{}
	switch models.OAuthProvider(c.Param("provider")) {
	case models.Slack:
		for _, field := range []string{"enterprise", "team"} {
			extra, ok := token.Extra(field).(map[string]any)
			router.Logger.WithField("field", field).WithField("extra", extra).Debug("Retrieved extra field from token")
			if !ok {
				router.Logger.WithField("field", field).Debug("Extra field is not a map[string]any")
				continue
			}

			originID, ok := extra["id"].(string)
			router.Logger.WithField("field", field).WithField("originID", originID).Debug("Retrieved origin ID from extra field")
			if !ok {
				router.Logger.WithField("field", field).Debug("Origin ID is not a string")
				continue
			}

			dbTokens = append(dbTokens, &models.Token{
				AccessToken:  token.AccessToken,
				RefreshToken: token.RefreshToken,
				ExpiresAt:    token.Expiry.Unix(),
				Origin:       "slack",
				OriginID:     originID,
			})
		}
	case models.Atlassian:
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
			dbTokens = append(dbTokens, &models.Token{
				AccessToken:  token.AccessToken,
				RefreshToken: token.RefreshToken,
				ExpiresAt:    token.Expiry.Unix(),
				Origin:       "atlassian",
				OriginID:     ar.URL,
			})
		}
	}

	whereTokens := [][]interface{}{}
	for _, dbToken := range dbTokens {
		whereTokens = append(whereTokens, []interface{}{dbToken.Origin, dbToken.OriginID})
	}

	accountUUID := sessions.Get[string](session, "account_id")
	var dbAccount *models.Account
	if accountUUID == "" {
		// Join preloading does not support the Tokens has-many association.
		// Find the owning account through matching token account UUIDs instead.
		err := router.Config.Database().Model(&models.Token{}).
			Select("account_uuid").
			Where("(origin, origin_id) IN ?", whereTokens).
			Where("account_uuid IS NOT NULL").First(&accountUUID).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to query account: %w", err))
			return
		}
	}

	if accountUUID != "" {
		dbAccount, err = gorm.G[*models.Account](router.Config.Database()).Where(models.Account{ID: uuid.MustParse(accountUUID)}).First(ctx)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to query account: %w", err))
			return
		}
	}

	if dbAccount == nil || errors.Is(err, gorm.ErrRecordNotFound) {
		err := gorm.G[*models.Account](router.Config.Database()).Create(ctx, &dbAccount)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to create account: %w", err))
			return
		}
	}

	for _, dbToken := range dbTokens {
		dbToken.AccountUUID = dbAccount.ID
		err := gorm.G[*models.Token](router.Config.Database(), clause.OnConflict{
			Columns:   []clause.Column{{Name: "origin"}, {Name: "origin_id"}},
			UpdateAll: true,
		}).Create(ctx, &dbToken)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to save token: %w", err))
			return
		}
	}

	sessions.Set(session, "account_id", dbAccount.ID.String())
	sessions.MustSave(session)

	router.Logger.WithField("account_id", dbAccount.ID).Debug("Saved account ID in session")
	c.Redirect(http.StatusFound, "/account/my")
}
