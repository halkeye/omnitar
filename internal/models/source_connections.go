package models

import (
	"github.com/google/uuid"

	"gorm.io/gorm"
)

type SourceConnection struct {
	gorm.Model

	ID     uuid.UUID `gorm:"type:uuid;primary_key;"`
	Type   string    `gorm:"uniqueIndex:idx_source_connections_type_team_id"`
	TeamID string    `gorm:"uniqueIndex:idx_source_connections_type_team_id"`
	Token  string
}

func (sc *SourceConnection) BeforeCreate(tx *gorm.DB) error {
	sc.ID = uuid.New() // Generates a new UUID
	return nil
}
