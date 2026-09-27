package models

import (
	"context"
	"errors"
	"strings"
	"uuid"

	"gorm.io/gorm"
)

type Account struct {
	gorm.Model

	ID uuid.UUID `gorm:"type:uuid;primary_key;"`
}

func (a *Account) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil() {
		a.ID = uuid.New() // Generates a new UUID
	}
	return nil
}

func (a *Account) GetTokenByOrigin(ctx context.Context, db *gorm.DB, origin string, defaultOrigin string) (*Token, error) {
	if origin == "" || origin == "auto" {
		origin = defaultOrigin
	}
	origin = strings.ToLower(strings.TrimSpace(origin))

	token, err := gorm.G[*Token](db).Where(Token{AccountUUID: a.ID, Origin: origin}).First(ctx)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	return token, nil
}
