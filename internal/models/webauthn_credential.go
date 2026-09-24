package models

import (
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"

	"gorm.io/gorm"
)

type WebAuthnCredential struct {
	gorm.Model
	ID   protocol.URLEncodedBase64 `gorm:"column:id;not null;type:varchar(96);serializer:json;primaryKey" json:"-"`
	Name string                    `gorm:"column:name;not null" json:"-"`

	AccountUUID uuid.UUID `gorm:"type:uuid;index"`
	Account     Account   `gorm:"foreignKey:AccountUUID;constraint:OnDelete:CASCADE;"`

	Credential webauthn.Credential `gorm:"column:credential;not null;serializer:json" json:"-"`
}

func (c *WebAuthnCredential) DisplayName() string {
	if c.Name != "" {
		return c.Name
	}
	if len(c.Credential.PublicKey) != 0 {
		return protocol.URLEncodedBase64(c.Credential.PublicKey).String()
	}
	return c.ID.String()
}
