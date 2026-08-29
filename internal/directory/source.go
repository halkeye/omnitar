package directory

import "context"

// Source knows how to fetch the full employee list from one upstream
// (Slack, LDAP, ...).
type Source interface {
	// Name identifies the source for logging/metrics, e.g. "slack" or "ldap".
	Name() string
	// Fetch returns every known person. Implementations should return an
	// error rather than a partial list on failure so callers can fall back
	// to another source.
	Fetch(ctx context.Context) error

	Lookup(ctx context.Context, hashedToken string) (Person, error)

	Len() int
}
