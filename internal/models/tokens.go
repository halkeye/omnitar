package models

import (
	"time"

	"github.com/google/uuid"

	"gorm.io/gorm"
)

type Token struct {
	ID           uuid.UUID      `gorm:"type:uuid;primarykey;"`
	AccountUUID  *uuid.UUID     `gorm:"type:uuid;index"`
	Account      *Account       `gorm:"foreignKey:AccountUUID;constraint:OnDelete:CASCADE;"`
	Origin       string         `gorm:"not null;uniqueIndex:idx_tokens_origin"` // slack
	OriginID     string         `gorm:"not null;uniqueIndex:idx_tokens_origin"` // slack=teamid
	AccountToken string         `gorm:"not null"`
	ExpiresAt    int64          `gorm:"not null;index"`
	RefreshToken string         `gorm:""`
	DeletedAt    gorm.DeletedAt `gorm:"index;uniqueIndex:idx_tokens_origin"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (t *Token) BeforeCreate(tx *gorm.DB) error {
	t.ID = uuid.New() // Generates a new UUID
	return nil
}
