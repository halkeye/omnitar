package models

import (
	"uuid"

	"gorm.io/gorm"

	"github.com/sirupsen/logrus"

	"github.com/halkeye/omnitar/internal/directory"
)

type Token struct {
	gorm.Model

	ID           uuid.UUID `gorm:"type:uuid;primarykey;"`
	AccountUUID  uuid.UUID `gorm:"type:uuid;index"`
	Account      *Account  `gorm:"foreignKey:AccountUUID;constraint:OnDelete:CASCADE;"`
	Origin       string    `gorm:"not null;uniqueIndex:idx_tokens_origin"` // slack
	OriginID     string    `gorm:"not null;uniqueIndex:idx_tokens_origin"` // slack=teamid
	AccessToken  string    `gorm:"not null"`
	ExpiresAt    int64     `gorm:"not null;index"`
	RefreshToken string    `gorm:""`
}

func (t *Token) BeforeCreate(tx *gorm.DB) error {
	t.ID = uuid.New() // Generates a new UUID
	return nil
}

func (t *Token) Source(logger *logrus.Logger) directory.Source {
	return directory.NewSlackSource(logger, t.OriginID, t.AccessToken)
}

func (t *Token) AsLog() logrus.Fields {
	return logrus.Fields{
		"token.id":            t.ID,
		"token.account_uuid":  t.AccountUUID,
		"token.origin":        t.Origin,
		"token.origin_id":     t.OriginID,
		"token.expires_at":    t.ExpiresAt,
		"token.refresh_token": t.RefreshToken,
	}
}
