package models

import (
	"crypto/rand"
	"fmt"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"

	"gorm.io/gorm"
)

type Account struct {
	gorm.Model

	ID                   uuid.UUID            `gorm:"type:uuid;primary_key;"`
	WebAuthnID_          []byte               `gorm:"column:webauthn_id"`
	Tokens               []Token              `gorm:"foreignKey:AccountUUID;constraint:OnDelete:CASCADE;"`
	_WebAuthnCredentials []WebAuthnCredential `gorm:"column:webauthn_credentials;foreignkey:AccountUUID;constraint:OnDelete:CASCADE;"`
}

func (a Account) GetTokenForSource(s string) *Token {
	for _, t := range a.Tokens {
		if t.Origin == s {
			return new(t)
		}
	}
	return nil
}

func (a *Account) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New() // Generates a new UUID
	}
	if len(a.WebAuthnID_) == 0 {
		buf := make([]byte, 64)
		_, err := rand.Read(buf)
		if err != nil {
			return fmt.Errorf("error while generating random bytes: %w", err)
		}
		a.WebAuthnID_ = buf[0:64]
	}
	return nil
}

func (a *Account) WebAuthnID() []byte {
	return a.WebAuthnID_
}

func (a *Account) WebAuthnName() string {
	return a.ID.String()
}

func (a *Account) WebAuthnDisplayName() string {
	return a.ID.String()
}

func (a *Account) WebAuthnCredentials() []webauthn.Credential {
	var c []webauthn.Credential
	for _, cred := range a._WebAuthnCredentials {
		c = append(c, cred.Credential)
	}
	return c
}
