package refresh

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/halkeye/omnitar/internal/directory"
)

type fakeSource struct {
	people []directory.Person
	err    error
	calls  int
}

func (f *fakeSource) Name() string { return "fake" }

func (f *fakeSource) Fetch(_ context.Context) error {
	f.calls++
	if f.err != nil {
		return f.err
	}
	return nil
}

func (f *fakeSource) Lookup(_ context.Context, hash string) (directory.Person, error) {
	for _, p := range f.people {
		if directory.MD5Hash(p.Email) == hash || directory.SHA256Hash(p.Email) == hash {
			return p, nil
		}
	}
	return directory.Person{}, nil
}

func (f *fakeSource) Len() int {
	return -1
}

func newTestLogger() *logrus.Logger {
	l := logrus.New()
	l.SetOutput(io.Discard)
	return l
}

func TestOnceSwapsStore(t *testing.T) {
	src := &fakeSource{people: []directory.Person{{Email: "a@example.com", Name: "A"}}}
	r := &Refresher{Source: src, Logger: newTestLogger()}

	if err := r.Once(context.Background()); err != nil {
		t.Fatalf("Once() error = %v", err)
	}
	if got, want := src.Len(), -1; got != want {
		t.Errorf("Store.Len() = %d, want %d", got, want)
	}
}

func TestOncePropagatesError(t *testing.T) {
	src := &fakeSource{err: errors.New("boom")}
	r := &Refresher{Source: src, Logger: newTestLogger()}

	if err := r.Once(context.Background()); err == nil {
		t.Fatal("Once() error = nil, want error")
	}
}

func TestRunPeriodicKeepsPreviousSnapshotOnFailure(t *testing.T) {
	src := &fakeSource{people: []directory.Person{{Email: "a@example.com", Name: "A"}}}
	r := &Refresher{Source: src, Interval: 5 * time.Millisecond, Logger: newTestLogger()}

	// Initial load succeeds.
	if err := r.Once(context.Background()); err != nil {
		t.Fatalf("Once() error = %v", err)
	}

	// Subsequent periodic fetches fail; the snapshot should remain intact.
	src.err = errors.New("boom")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	err := r.RunPeriodic(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("RunPeriodic() error = %v, want context.DeadlineExceeded", err)
	}
	if src.calls < 2 {
		t.Errorf("expected at least 2 fetch attempts, got %d", src.calls)
	}
	if got, want := src.Len(), -1; got != want {
		t.Errorf("Store.Len() = %d, want %d (should keep last good snapshot)", got, want)
	}
}
