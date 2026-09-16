package models

import (
	"github.com/google/uuid"

	"gorm.io/gorm"
)

type Account struct {
	gorm.Model

	ID     uuid.UUID `gorm:"type:uuid;primary_key;"`
	Tokens []Token   `gorm:"foreignKey:AccountUUID;constraint:OnDelete:CASCADE;"`
}

func (a *Account) BeforeCreate(tx *gorm.DB) error {
	a.ID = uuid.New() // Generates a new UUID
	return nil
}
