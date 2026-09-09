// Package config loads runtime configuration from environment variables.
package config

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/halkeye/omnitar/internal/directory"
	"github.com/halkeye/omnitar/internal/refresh"

	"github.com/caarlos0/env/v11"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// config holds all runtime configuration for the service.
type config struct {
	AppEnv             string            `env:"APP_ENV,required" envDefault:"development"`
	DatabaseURL        string            `env:"DATABASE_URL"`
	SlackClientID_     string            `env:"SLACK_CLIENT_ID"`
	SlackClientSecret_ string            `env:"SLACK_CLIENT_SECRET"`
	SlackBotTokens     map[string]string `env:"SLACK_BOT_TOKENS,required"`
	Port               string            `env:"PORT,required" envDefault:"8080"`
	RefreshInterval    time.Duration     `env:"REFRESH_INTERVAL,required" envDefault:"2h"`
	StartupTimeout     time.Duration     `env:"STARTUP_TIMEOUT,required" envDefault:"2m"`
	LogLevel           string            `env:"LOG_LEVEL,required" envDefault:"info"`

	db *gorm.DB

	logger *logrus.Logger

	mu             sync.RWMutex
	sources        map[string]directory.Source
	refreshCancels map[string]context.CancelFunc
}

type SourceConnection struct {
	gorm.Model
	Type   string
	TeamID string
	Token  string
}

// Load reads config from the environment, applying defaults for anything
// unset and validating required fields.
func Load() (*config, error) {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	cfg, err := env.ParseAs[config]()
	cfg.logger = logger
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if level, err := logrus.ParseLevel(cfg.LogLevel); err != nil {
		logger.WithError(err).Warn("invalid LOG_LEVEL, defaulting to info")
	} else {
		logger.SetLevel(level)
	}

	cfg.logger = logger
	cfg.sources = map[string]directory.Source{}
	cfg.refreshCancels = map[string]context.CancelFunc{}

	for slackOrgID, token := range cfg.SlackBotTokens {
		logger.WithField("slackOrgID", slackOrgID).Info("adding Slack source")
		if err := cfg.AddSource(slackOrgID, directory.NewSlackSource(logger, slackOrgID, token)); err != nil {
			return nil, fmt.Errorf("failed to start refresher for Slack source %s: %w", slackOrgID, err)
		}
	}

	if cfg.DatabaseURL != "" {
		logger.Debug("connecting to database")
		db, err := gorm.Open(postgres.New(postgres.Config{
			DSN:                  cfg.DatabaseURL,
			PreferSimpleProtocol: true, // disables implicit prepared statement usage
		}), &gorm.Config{})
		if err != nil {
			return nil, fmt.Errorf("failed to connect to database: %w", err)
		}
		cfg.db = db

		// Migrate the schema
		db.AutoMigrate(&SourceConnection{})

		sourceConnections, err := gorm.G[SourceConnection](db).Find(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to fetch source connections from database: %w", err)
		}
		for _, sourceConnection := range sourceConnections {
			if sourceConnection.Type == "slack" {
				if err := cfg.AddSource(sourceConnection.TeamID, directory.NewSlackSource(logger, sourceConnection.TeamID, sourceConnection.Token)); err != nil {
					return nil, fmt.Errorf("failed to start refresher for Slack source %s: %w", sourceConnection.TeamID, err)
				}
			}
		}
	}

	return &cfg, nil
}

var _ Config = &config{}

type Config interface {
	IsDev() bool

	Logger() *logrus.Logger

	SlackClientID() string
	SlackClientSecret() string

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

func (c *config) Source(sourceID string) directory.Source {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sources[sourceID]
}

func (c *config) SaveSourceConnection(ctx context.Context, sourceType string, sourceID string, token string) error {
	conn := &SourceConnection{Type: sourceType, TeamID: sourceID, Token: token}
	if c.db != nil {
		err := gorm.G[SourceConnection](c.db).Create(ctx, conn)
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
