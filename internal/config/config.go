// Package config loads runtime configuration from environment variables.
package config

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"time"

	"github.com/halkeye/omnitar/internal/logger"
	"github.com/halkeye/omnitar/internal/models"

	"github.com/caarlos0/env/v11"
	"github.com/glebarez/sqlite"
	"github.com/sirupsen/logrus"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// config holds all runtime configuration for the service.
type config struct {
	AppEnv                 string            `env:"APP_ENV,required" envDefault:"development"`
	SlackClientID_         string            `env:"SLACK_CLIENT_ID"`
	SlackClientSecret_     string            `env:"SLACK_CLIENT_SECRET"`
	AtlassianClientID_     string            `env:"ATLASSIAN_CLIENT_ID"`
	AtlassianClientSecret_ string            `env:"ATLASSIAN_CLIENT_SECRET"`
	SlackBotTokens         map[string]string `env:"SLACK_BOT_TOKENS,required"`
	Port                   string            `env:"PORT,required" envDefault:"8080"`
	StartupTimeout         time.Duration     `env:"STARTUP_TIMEOUT,required" envDefault:"2m"`
	LogLevel               string            `env:"LOG_LEVEL,required" envDefault:"info"`
	SessionKey_            string            `env:"SESSION_KEY,required" envDefault:"omnitar-session-key"`
	Database_              *gorm.DB          `env:"DATABASE_URL" envDefault:""`
}

func (c *config) SetupDB(ctx context.Context) error {
	var err error
	ll := logger.FromContext(ctx)
	ll.Debug("connecting to database")
	// Migrate the schema
	err = c.Database_.AutoMigrate(&models.Account{}, &models.Token{})
	if err != nil {
		return fmt.Errorf("failed to migrate schema: %w", err)
	}
	ll.Debug("migration done")

	accounts, err := gorm.G[models.Account](c.Database_).Find(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch accounts from database: %w", err)
	}

	for _, account := range accounts {
		if err := account.BeforeCreate(c.Database_); err != nil {
			return fmt.Errorf("failed to run BeforeCreate for account %s: %w", account.ID, err)
		}

		if _, err := gorm.G[models.Account](c.Database_).Updates(ctx, account); err != nil {
			return fmt.Errorf("failed to update account %s: %w", account.ID, err)
		}
	}
	return nil
}

func New() *config {
	cfg := &config{}

	return cfg
}

func parseDatabase(v string) (any, error) {
	if v == "" {
		return nil, nil
	}
	parsedURL, err := url.Parse(v)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	switch parsedURL.Scheme {
	case "postgres", "postgresql":
		db, err := gorm.Open(postgres.New(postgres.Config{DSN: v, PreferSimpleProtocol: true}), &gorm.Config{})
		if err != nil {
			return nil, fmt.Errorf("failed to connect to database: %w", err)
		}
		return *db, nil
	case "sqlite", "sqlite3":
		// remove the sqlite, then the ://
		dsn := v[len(parsedURL.Scheme)+3:]
		db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		if err != nil {
			return nil, fmt.Errorf("failed to connect to database: %w", err)
		}
		return *db, nil
	}
	return nil, errors.New("unknown type")
}

func Load() (*config, error) {
	cfg := New()
	err := env.ParseWithOptions(cfg, env.Options{
		FuncMap: map[reflect.Type]env.ParserFunc{
			reflect.TypeFor[gorm.DB](): parseDatabase,
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if level, err := logrus.ParseLevel(cfg.LogLevel); err != nil {
		logger.DefaultLogger.WithError(err).Warn("invalid LOG_LEVEL, defaulting to info")
	} else {
		logger.DefaultLogger.SetLevel(level)
	}

	if cfg.Database_ != nil {
		// cfg.Database_.Logger = gormlogrus{
		// 	logger:                cfg.logger,
		// 	SourceField:           "source",
		// 	SkipErrRecordNotFound: true,
		// }

		if err := cfg.SetupDB(context.Background()); err != nil {
			return nil, fmt.Errorf("failed to setup database: %w", err)
		}

	}

	return cfg, nil
}

var _ Config = &config{}

type Config interface {
	io.Closer

	IsDev() bool

	SlackClientID() string
	SlackClientSecret() string

	AtlassianClientID() string
	AtlassianClientSecret() string

	SessionKey() string
	Database() *gorm.DB
}

func (c *config) IsDev() bool {
	return c.AppEnv == "development"
}

func (c *config) SlackClientID() string {
	return c.SlackClientID_
}

func (c *config) SlackClientSecret() string {
	return c.SlackClientSecret_
}

func (c *config) AtlassianClientID() string {
	return c.AtlassianClientID_
}

func (c *config) AtlassianClientSecret() string {
	return c.AtlassianClientSecret_
}

func (c *config) SessionKey() string {
	return c.SessionKey_
}

func (c *config) Database() *gorm.DB {
	return c.Database_
}

// Close closes the database connection, if
// one was opened via SetupDB.
func (c *config) Close() error {
	if c.Database_ == nil {
		return nil
	}

	sqlDB, err := c.Database_.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying database connection: %w", err)
	}
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("failed to close database connection: %w", err)
	}
	return nil
}
