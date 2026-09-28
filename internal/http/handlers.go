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

	"github.com/gin-contrib/location/v2"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/halkeye/omnitar/internal/database"
	"github.com/halkeye/omnitar/internal/http/gintemplrenderer"
	"github.com/halkeye/omnitar/internal/logger"
	"github.com/halkeye/omnitar/internal/models"
	"github.com/halkeye/omnitar/internal/providers"
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
	ctx := c.Request.Context()
	ll := logger.FromContext(ctx)

	account := getTyped[models.Account](c, "account")
	token, err := account.GetTokenByOrigin(ctx, database.FromContext(ctx), c.Param("source"), "slack")
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to get token: %w", err))
		return
	}
	if token == nil {
		ll.WithField("accountId", account.ID).Debug("no slack token for account")
		c.Status(http.StatusNotFound)
		return
	}

	source := token.PersonSource(ctx)
	if source == nil {
		ll.WithFields(token.AsLog()).Debug("unknown Slack org ID")
		c.Status(http.StatusNotFound)
		return
	}

	p, err := source.LookupPerson(ctx, c.Param("profileIdentifier"))
	if err != nil {
		c.Error(err)
		return
	}

	if p == nil {
		if c.Query("d") == "404" {
			c.Status(http.StatusNotFound)
			return
		}

		c.Header("Cache-Control", "public, max-age=3600, no-transform")
		c.Data(http.StatusOK, "image/svg+xml", router.defaultAvatar)
		return
	}

	c.Redirect(http.StatusFound, p.AvatarURL)
}

func (router *Deps) handlerIndexPage(c *gin.Context) {
	if router.Config.IsDev() {
		if c.Query("token") != "" {
			gin.WrapH(router.StaticHandler)(c)
			return
		}
	}
	r := gintemplrenderer.New(c.Request.Context(), http.StatusOK, templates.Index())
	c.Render(http.StatusOK, r)
}

func (router *Deps) handlerMyAccountPage(c *gin.Context) {
	var err error

	ctx := c.Request.Context()
	dbAccount := getTyped[models.Account](c, "account")

	tokens, err := gorm.G[*models.Token](database.FromContext(ctx)).Where(models.Token{AccountUUID: dbAccount.ID}).Find(c.Request.Context())
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to load tokens: %w", err))
		return
	}

	r := gintemplrenderer.New(c.Request.Context(), http.StatusOK, templates.AccountMyPage(&dbAccount, tokens))
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
	ctx := c.Request.Context()
	ll := logger.FromContext(ctx)

	account := getTyped[models.Account](c, "account")
	token, err := account.GetTokenByOrigin(ctx, database.FromContext(ctx), c.Param("source"), "slack")
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to get token: %w", err))
		return
	}
	if token == nil {
		ll.WithField("accountId", account.ID).Debug("no slack token for account")
		c.Status(http.StatusNotFound)
		return
	}

	source := token.PersonSource(ctx)
	if source == nil {
		ll.WithFields(token.AsLog()).Debug("unknown Slack org ID")
		c.Status(http.StatusNotFound)
		return
	}

	ll = ll.WithFields(token.AsLog())

	email := c.Param("email")
	email = strings.TrimSpace(email)
	email = strings.ToLower(email)

	ll = ll.WithField("email", email)
	p, err := source.LookupPerson(c.Request.Context(), email)
	if err != nil && !errors.Is(err, &models.NotFoundError{}) {
		ll.Debug("error fetching profile")
		c.Error(err)
		return
	}

	if errors.Is(err, &models.NotFoundError{}) {
		ll.Debug("profile not found")
		c.Status(http.StatusNotFound)
		return
	}

	c.JSON(http.StatusOK, p)
}

func (router *Deps) issueHandler(c *gin.Context) {
	ctx := c.Request.Context()
	ll := logger.FromContext(ctx)
	account := getTyped[models.Account](c, "account")

	token, err := account.GetTokenByOrigin(ctx, database.FromContext(ctx), c.Param("source"), "atlassian")
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to get token: %w", err))
		return
	}

	if token == nil {
		ll.WithField("accountId", account.ID).Debug("no slack token for account")
		c.Status(http.StatusNotFound)
		return
	}

	source := token.IssueSource(ctx)
	if source == nil {
		ll.WithFields(token.AsLog()).Debug("unknown issue ID")
		c.Status(http.StatusNotFound)
		return
	}

	ll = ll.WithFields(token.AsLog())

	id := c.Param("id")

	ll = ll.WithField("id", id)
	p, err := source.LookupIssue(ctx, id)
	if err != nil {
		ll.Debug("error fetching profile")
		c.Error(err)
		return
	}

	if p == nil {
		ll.Debug("profile not found")
		c.Status(http.StatusNotFound)
		return
	}

	c.JSON(http.StatusOK, p)
}

func (router *Deps) deleteTokenHandler(c *gin.Context) {
	ctx := c.Request.Context()
	ll := logger.FromContext(ctx)

	session := sessions.Default(c)

	dbAccount := getTyped[models.Account](c, "account")

	tokenID := c.Param("token")
	if tokenID == "" {
		ll.Debug("No token ID provided")
		c.Status(http.StatusNotFound)
		return
	}

	_, err := gorm.G[models.Token](database.FromContext(ctx)).Where(models.Token{ID: uuid.MustParse(tokenID), AccountUUID: dbAccount.ID}).Delete(ctx)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to delete token: %w", err))
		return
	}

	session.AddFlash("Token deleted successfully.")
	c.Status(http.StatusNoContent)
}

func (router *Deps) handlerProvider(c *gin.Context) {
	ctx := c.Request.Context()
	session := sessions.Default(c)
	state, err := generateRandomState()
	if err != nil {
		c.String(http.StatusInternalServerError, "Unable to generate state value")
		return
	}

	oauth2Config := providers.FromContext(ctx).Get(providers.OAuthProvider(c.Param("provider")))
	if oauth2Config == nil {
		c.AbortWithError(http.StatusBadRequest, errors.New("unsupported provider"))
		return
	}

	sessions.Set(session, "state_"+state, time.Now().Format(time.RFC3339))
	sessions.MustSave(session)

	callbackURL := location.Get(c)
	callbackURL.Path = "/auth/" + c.Param("provider") + "/callback"
	oauth2Config.Config.RedirectURL = callbackURL.String()
	authURL := oauth2Config.Config.AuthCodeURL(state, append(oauth2Config.Options, oauth2.ApprovalForce)...)
	c.Redirect(http.StatusFound, authURL)
}

func (router *Deps) handlerProviderCallback(c *gin.Context) {
	ctx := c.Request.Context()
	ll := logger.FromContext(ctx)
	session := sessions.Default(c)

	oauth2Config := providers.FromContext(ctx).Get(providers.OAuthProvider(c.Param("provider")))
	if oauth2Config == nil {
		c.AbortWithError(http.StatusBadRequest, errors.New("unsupported provider"))
		return
	}

	callbackURL := location.Get(c)
	callbackURL.Path = "/auth/" + c.Param("provider") + "/callback"
	oauth2Config.Config.RedirectURL = callbackURL.String()

	// Retrieve and verify state
	state := c.Query("state")
	stateVal := sessions.Get[string](session, "state_"+state)
	ll.WithField("state", stateVal).Debug("OAuth callback received")
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
	token, err := oauth2Config.Config.Exchange(ctx, code, oauth2Config.Options...)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("Unable to exchange access token: %w", err))
		return
	}

	dbTokens := []*models.Token{}
	switch providers.OAuthProvider(c.Param("provider")) {
	case providers.Slack:
		for _, field := range []string{"enterprise", "team"} {
			extra, ok := token.Extra(field).(map[string]any)
			ll.WithField("field", field).WithField("extra", extra).Debug("Retrieved extra field from token")
			if !ok {
				ll.WithField("field", field).Debug("Extra field is not a map[string]any")
				continue
			}

			originID, ok := extra["id"].(string)
			ll.WithField("field", field).WithField("originID", originID).Debug("Retrieved origin ID from extra field")
			if !ok {
				ll.WithField("field", field).Debug("Origin ID is not a string")
				continue
			}

			dbTokens = append(dbTokens, &models.Token{
				AccessToken:  token.AccessToken,
				RefreshToken: token.RefreshToken,
				ExpiresAt:    token.Expiry,
				Origin:       "slack",
				OriginID:     originID,
			})
		}
	case providers.Atlassian:
		client := oauth2Config.Config.Client(ctx, token)
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
				ExpiresAt:    token.Expiry,
				Origin:       "atlassian",
				OriginID:     ar.ID,
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
		err := database.FromContext(ctx).Model(&models.Token{}).
			Select("account_uuid").
			Where("(origin, origin_id) IN ?", whereTokens).
			Where("account_uuid IS NOT NULL").First(&accountUUID).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to query account: %w", err))
			return
		}
	}

	if accountUUID != "" {
		dbAccount, err = gorm.G[*models.Account](database.FromContext(ctx)).Where(models.Account{ID: uuid.MustParse(accountUUID)}).First(ctx)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to query account: %w", err))
			return
		}
	}

	if dbAccount == nil || errors.Is(err, gorm.ErrRecordNotFound) {
		err := gorm.G[*models.Account](database.FromContext(ctx)).Create(ctx, &dbAccount)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to create account: %w", err))
			return
		}
	}

	for _, dbToken := range dbTokens {
		dbToken.AccountUUID = dbAccount.ID
		err := gorm.G[*models.Token](database.FromContext(ctx), clause.OnConflict{
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

	ll.WithField("account_id", dbAccount.ID).Debug("Saved account ID in session")
	c.Redirect(http.StatusFound, "/account/my")
}
