package config_test

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/halkeye/omnitar/internal/config"
)

// startPostgres spins up a disposable Postgres container for the test and
// returns a DSN pointed at it. The container is terminated via t.Cleanup.
func startPostgres(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx,
		"postgres:17-alpine",
		tcpostgres.WithDatabase("omnitar"),
		tcpostgres.WithUsername("omnitar"),
		tcpostgres.WithPassword("omnitar"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			t.Errorf("failed to terminate postgres container: %v", err)
		}
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get postgres connection string: %v", err)
	}
	return dsn
}

// TestSetupDBWithPostgres exercises SetupDB against a real Postgres
// instance: migrations run, a source connection can be saved, and
// re-authorizing the same team upserts rather than violating the unique
// (type, team_id) index.
func TestSetupDBWithPostgres(t *testing.T) {
	dsn := startPostgres(t)
	ctx := t.Context()

	cfg := config.New()
	cfg.Database_, _ = gorm.Open(postgres.Open(dsn))
	if err := cfg.SetupDB(ctx); err != nil {
		t.Fatalf("SetupDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := cfg.Close(); err != nil {
			t.Errorf("cfg.Close() error = %v", err)
		}
	})

	if err := cfg.SaveSourceConnection(ctx, "slack", "T123", "xoxb-first"); err != nil {
		t.Fatalf("SaveSourceConnection() error = %v", err)
	}

	// Re-authorizing the same team should upsert the token, not fail on the
	// unique index, against a real Postgres ON CONFLICT clause.
	if err := cfg.SaveSourceConnection(ctx, "slack", "T123", "xoxb-second"); err != nil {
		t.Fatalf("SaveSourceConnection() (re-authorized) error = %v", err)
	}
}

// TestSetupDBWithPostgresReloadsExistingSources confirms that restarting
// against a Postgres database with existing source connections re-registers
// them as sources.
func TestSetupDBWithPostgresReloadsExistingSources(t *testing.T) {
	dsn := startPostgres(t)
	ctx := t.Context()

	first := config.New()
	first.Database_, _ = gorm.Open(postgres.Open(dsn))
	if err := first.SetupDB(ctx); err != nil {
		t.Fatalf("SetupDB() error = %v", err)
	}
	if err := first.SaveSourceConnection(ctx, "slack", "T456", "xoxb-token"); err != nil {
		t.Fatalf("SaveSourceConnection() error = %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	second := config.New()
	second.Database_, _ = gorm.Open(postgres.Open(dsn))
	if err := second.SetupDB(ctx); err != nil {
		t.Fatalf("SetupDB() (reload) error = %v", err)
	}
	t.Cleanup(func() {
		if err := second.Close(); err != nil {
			t.Errorf("cfg.Close() error = %v", err)
		}
	})

	if source := second.Source("T456"); source == nil {
		t.Fatalf("Source(%q) = nil, want the previously saved source connection to be reloaded", "T456")
	}
}
