package models

import (
	"github.com/google/uuid"

	"gorm.io/gorm"
)

type Token struct {
	gorm.Model

	ID           uuid.UUID  `gorm:"type:uuid;primary_key;"`
	AccountUUID  *uuid.UUID `gorm:"type:uuid;index"`
	Account      *Account   `gorm:"foreignKey:AccountUUID;constraint:OnDelete:CASCADE;"`
	Origin       string     `gorm:"not null;uniqueIndex:idx_tokens_origin"` // slack
	OriginID     string     `gorm:"not null;uniqueIndex:idx_tokens_origin"` // slack=teamid
	AccountToken string     `gorm:"not null"`
	ExpiresAt    int64      `gorm:"not null;index"`
	RefreshToken string     `gorm:""`
}

func (t *Token) BeforeCreate(tx *gorm.DB) error {
	t.ID = uuid.New() // Generates a new UUID
	return nil
}
