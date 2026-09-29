package models

import (
	"context"
	"fmt"
	"time"
	"uuid"

	"golang.org/x/oauth2"
	"gorm.io/gorm"

	"github.com/halkeye/omnitar/internal/database"
	"github.com/halkeye/omnitar/internal/logger"
	"github.com/halkeye/omnitar/internal/providers"
)

type Token struct {
	gorm.Model

	ID           uuid.UUID `gorm:"type:uuid;primarykey;"`
	AccountUUID  uuid.UUID `gorm:"type:uuid;index"`
	Account      *Account  `gorm:"foreignKey:AccountUUID;constraint:OnDelete:CASCADE;"`
	Origin       string    `gorm:"not null;uniqueIndex:idx_tokens_origin"` // slack
	OriginID     string    `gorm:"not null;uniqueIndex:idx_tokens_origin"` // slack=teamid
	AccessToken  string    `gorm:"not null"`
	ExpiresAt    time.Time `gorm:"not null;index"`
	RefreshToken string    `gorm:""`
	TokenType    string    `gorm:""`
}

func (t *Token) UpdateFromOauth(ctx context.Context, newToken *oauth2.Token) error {
	db := database.FromContext(ctx)
	_, err := gorm.G[Token](db).Where("id = ?", t.ID).Updates(ctx, Token{
		ID:           t.ID,
		AccessToken:  newToken.AccessToken,
		RefreshToken: newToken.RefreshToken,
		ExpiresAt:    newToken.Expiry,
		TokenType:    newToken.TokenType,
	})
	if err != nil {
		return fmt.Errorf("failed to update token in database: %w", err)
	}

	t.AccessToken = newToken.AccessToken
	t.RefreshToken = newToken.RefreshToken
	t.ExpiresAt = newToken.Expiry

	return nil
}

func FromOauthToken(accountUUID uuid.UUID, origin string, originID string, oauthToken *oauth2.Token) *Token {
	return &Token{
		AccountUUID:  accountUUID,
		Origin:       origin,
		OriginID:     originID,
		AccessToken:  oauthToken.AccessToken,
		RefreshToken: oauthToken.RefreshToken,
		ExpiresAt:    oauthToken.Expiry,
		TokenType:    oauthToken.TokenType,
	}
}

func (t *Token) GetAccessToken() string {
	return t.AccessToken
}

func (t *Token) AsOAuthToken() *oauth2.Token {
	return &oauth2.Token{
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
		Expiry:       t.ExpiresAt,
		TokenType:    t.TokenType,
	}
}

func (t *Token) BeforeCreate(tx *gorm.DB) error {
	t.ID = uuid.New() // Generates a new UUID
	return nil
}

func (t *Token) PersonSource(ctx context.Context) PersonSource {
	switch t.Origin {
	case "slack":
		return NewSlackSource(t.OriginID, t)
	default:
		panic("unknown source type: " + t.Origin)
	}
}

func (t *Token) IssueSource(ctx context.Context) IssueSource {
	switch t.Origin {
	case "atlassian":
		return NewAtlassianSource(t.OriginID, t)
	default:
		panic("unknown source type: " + t.Origin)
	}
}

func (t *Token) AsLog() logger.Fields {
	return logger.Fields{
		"token.id":            t.ID,
		"token.account_uuid":  t.AccountUUID,
		"token.origin":        t.Origin,
		"token.origin_id":     t.OriginID,
		"token.expires_at":    t.ExpiresAt,
		"token.refresh_token": t.RefreshToken,
	}
}

func (t *Token) OAuthConfig(ctx context.Context) *providers.SourceOauthContainer {
	switch t.Origin {
	case "atlassian":
		return providers.FromContext(ctx).Get(providers.Atlassian)
	case "slack":
		return providers.FromContext(ctx).Get(providers.Slack)
	default:
		panic("unknown source type: " + t.Origin)
	}
}
