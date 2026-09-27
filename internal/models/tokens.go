package models

import (
	"context"
	"fmt"
	"time"
	"uuid"

	"golang.org/x/oauth2"
	"gorm.io/gorm"

	"github.com/halkeye/omnitar/internal/logger"
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
}

func (t *Token) AsOAuthToken() *oauth2.Token {
	return &oauth2.Token{
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
		Expiry:       t.ExpiresAt,
	}
}

func (t *Token) BeforeCreate(tx *gorm.DB) error {
	t.ID = uuid.New() // Generates a new UUID
	return nil
}

func (t *Token) PersonSource(ctx context.Context) PersonSource {
	switch t.Origin {
	case "slack":
		return NewSlackSource(t.OriginID, &oauth2.Token{AccessToken: t.AccessToken})
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
		"token.expires_at":    fmt.Sprintf("%d", t.ExpiresAt),
		"token.refresh_token": t.RefreshToken,
	}
}
