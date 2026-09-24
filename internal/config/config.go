// Package config loads runtime configuration from environment variables.
package config

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/halkeye/omnitar/internal/directory"
	"github.com/halkeye/omnitar/internal/models"
	"github.com/halkeye/omnitar/internal/refresh"

	"github.com/caarlos0/env/v11"
	"github.com/glebarez/sqlite"
	"github.com/sirupsen/logrus"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	RefreshInterval        time.Duration     `env:"REFRESH_INTERVAL,required" envDefault:"2h"`
	StartupTimeout         time.Duration     `env:"STARTUP_TIMEOUT,required" envDefault:"2m"`
	LogLevel               string            `env:"LOG_LEVEL,required" envDefault:"info"`
	SessionKey_            string            `env:"SESSION_KEY,required" envDefault:"omnitar-session-key"`
	Database_              *gorm.DB          `env:"DATABASE_URL" envDefault:""`

	logger *logrus.Logger

	mu             sync.RWMutex
	sources        map[string]directory.Source
	refreshCancels map[string]context.CancelFunc
}

func (c *config) SetupDB(ctx context.Context) error {
	var err error
	c.logger.Debug("connecting to database")
	// Migrate the schema
	err = c.Database_.AutoMigrate(&models.SourceConnection{}, &models.Account{}, &models.Token{}, &models.WebAuthnCredential{})
	if err != nil {
		return fmt.Errorf("failed to migrate schema: %w", err)
	}
	c.logger.Debug("migration done")

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

	// sourceConnections, err := gorm.G[models.SourceConnection](c.Database_).Find(ctx)
	// if err != nil {
	// 	return fmt.Errorf("failed to fetch source connections from database: %w", err)
	// }
	// for _, sourceConnection := range sourceConnections {
	// 	if sourceConnection.Type == "slack" {
	// 		if err := c.AddSource(sourceConnection.TeamID, directory.NewSlackSource(c.logger, sourceConnection.TeamID, sourceConnection.Token)); err != nil {
	// 			return fmt.Errorf("failed to start refresher for Slack source %s: %w", sourceConnection.TeamID, err)
	// 		}
	// 	}
	// }
	return nil
}

func New() *config {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetOutput(os.Stdout)

	cfg := &config{}
	cfg.logger = logger
	cfg.sources = map[string]directory.Source{}
	cfg.refreshCancels = map[string]context.CancelFunc{}

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
		cfg.logger.WithError(err).Warn("invalid LOG_LEVEL, defaulting to info")
	} else {
		cfg.logger.SetLevel(level)
	}

	for slackOrgID, token := range cfg.SlackBotTokens {
		cfg.logger.WithField("slackOrgID", slackOrgID).Info("adding Slack source")
		if err := cfg.AddSource(slackOrgID, directory.NewSlackSource(cfg.logger, slackOrgID, token)); err != nil {
			return nil, fmt.Errorf("failed to start refresher for Slack source %s: %w", slackOrgID, err)
		}
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

	Logger() *logrus.Logger

	SlackClientID() string
	SlackClientSecret() string

	AtlassianClientID() string
	AtlassianClientSecret() string

	SessionKey() string
	Database() *gorm.DB

	FindOrCreateTokenAndAccount(ctx context.Context, accountUUID string, token models.Token) (models.Token, error)

	Source(sourceID string) directory.Source
	AddSource(sourceID string, source directory.Source) error
	StartRefresher(sourceID string) error
	SaveSourceConnection(ctx context.Context, sourceType string, sourceID string, token string) error
}

func (c *config) Logger() *logrus.Logger {
	return c.logger
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

func (c *config) Source(sourceID string) directory.Source {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sources[sourceID]
}

// Close stops any running refreshers and closes the database connection, if
// one was opened via SetupDB.
func (c *config) Close() error {
	c.mu.Lock()
	for sourceID, cancel := range c.refreshCancels {
		cancel()
		delete(c.refreshCancels, sourceID)
	}
	c.mu.Unlock()

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

func (c *config) SaveSourceConnection(ctx context.Context, sourceType string, sourceID string, token string) error {
	conn := &models.SourceConnection{Type: sourceType, TeamID: sourceID, Token: token}
	if c.Database_ != nil {
		// Re-authorizing an already-connected team hits the same
		// (type, team_id), so upsert the token instead of erroring on the
		// unique index.
		onConflict := clause.OnConflict{
			Columns:   []clause.Column{{Name: "type"}, {Name: "team_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"token"}),
		}
		err := gorm.G[models.SourceConnection](c.Database_, onConflict).Create(ctx, conn)
		if err != nil {
			return fmt.Errorf("failed to save source connection: %w", err)
		}
	}
	return nil
}

func (c *config) AddSource(sourceID string, source directory.Source) error {
	c.mu.Lock()
	if cancel, ok := c.refreshCancels[sourceID]; ok {
		cancel()
		delete(c.refreshCancels, sourceID)
	}
	c.sources[sourceID] = source
	c.mu.Unlock()

	return c.StartRefresher(sourceID)
}

func (c *config) StartRefresher(sourceID string) error {
	ctx, cancel := context.WithCancel(context.Background())

	c.logger.WithField("sourceID", sourceID).Info("starting refresher for source")

	source := c.Source(sourceID)
	if source == nil {
		cancel()
		return fmt.Errorf("source %s not found", sourceID)
	}

	c.mu.Lock()
	if oldCancel, ok := c.refreshCancels[sourceID]; ok {
		oldCancel()
	}
	c.refreshCancels[sourceID] = cancel
	c.mu.Unlock()

	if c.RefreshInterval > 0 {
		refresher := &refresh.Refresher{Source: source, Interval: c.RefreshInterval, Logger: c.logger}
		go func() {
			startupCtx, cancelStartup := context.WithTimeout(ctx, c.StartupTimeout)
			err := initialLoad(startupCtx, source, refresher, c.logger)
			cancelStartup()

			if err != nil {
				c.logger.WithError(err).Fatal("initial directory load from Slack failed, giving up")
			}

			c.logger.WithFields(logrus.Fields{"people": source.Len()}).Info("initial directory load complete")
			if err := refresher.RunPeriodic(ctx); err != nil && !errors.Is(err, context.Canceled) {
				c.logger.WithError(err).Error("refresh loop exited unexpectedly")
			}
		}()
	}

	return nil
}

// initialLoad resolves the Slack team ID and performs the first
// fetch-and-swap, retrying with exponential backoff until both succeed or
// ctx is done.
func initialLoad(
	ctx context.Context,
	source directory.Source,
	refresher *refresh.Refresher,
	logger *logrus.Logger,
) error {
	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		err := refresher.Once(ctx)
		if err == nil {
			return nil
		}

		logger.WithError(err).Warn("initial Slack load failed, retrying")
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func (c *config) FindOrCreateTokenAndAccount(ctx context.Context, accountUUID string, token models.Token) (models.Token, error) {
	dbToken := models.Token{}

	c.logger.WithFields(logrus.Fields{
		"token.origin":   token.Origin,
		"token.originID": token.OriginID,
		"accountUUID":    accountUUID,
	}).Debug("Finding or creating token")

	result := c.Database_.
		Where(models.Token{Origin: token.Origin, OriginID: token.OriginID}).
		Attrs(token).
		FirstOrCreate(&dbToken)
	if result.Error != nil {
		return models.Token{}, fmt.Errorf("failed to retrieve or initialize token: %w", result.Error)
	}
	c.logger.WithField("dbToken", dbToken).Debug("Retrieved or initialized token")
	if accountUUID == "" && dbToken.AccountUUID != nil {
		accountUUID = dbToken.AccountUUID.String()
	}

	// if no account was found/created, create a new one
	if accountUUID == "" {
		dbAccount := &models.Account{}
		err := gorm.G[models.Account](c.Database_).Create(ctx, dbAccount)
		if err != nil {
			return models.Token{}, fmt.Errorf("failed to create account: %w", err)
		}
		accountUUID = dbAccount.ID.String()
	}

	// Make sure the token is associated with the right account now
	dbToken.AccountUUID = new(uuid.MustParse(accountUUID))
	result = c.Database_.Save(&dbToken)
	if result.Error != nil {
		return models.Token{}, fmt.Errorf("failed to save token with associated account: %w", result.Error)
	}

	return dbToken, nil
}
