package directory

import "context"

var _ error = &NotFoundError{}

type NotFoundError struct{}

func (e *NotFoundError) Error() string {
	return "not found"
}

// Source knows how to fetch the full employee list from one upstream
// (Slack, LDAP, ...).
type Source interface {
	// Name identifies the source for logging/metrics, e.g. "slack" or "ldap".
	Name() string

	Lookup(ctx context.Context, key string) (Person, error)

	Len() int
}
