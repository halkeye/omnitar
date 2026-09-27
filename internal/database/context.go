package database

import (
	"context"

	"gorm.io/gorm"
)

type Database = *gorm.DB

type contextKey string

var databaseKey contextKey = "db"

func WithDatabase(ctx context.Context, db Database) context.Context {
	return context.WithValue(ctx, databaseKey, db)
}

func FromContext(ctx context.Context) Database {
	val, ok := ctx.Value(databaseKey).(Database)
	if !ok {
		panic("no db on context")
	}
	return val
}
