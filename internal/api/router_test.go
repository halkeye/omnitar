package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sirupsen/logrus/hooks/test"

	"github.com/halkeye/omnitar/internal/config"
	"github.com/halkeye/omnitar/internal/directory"
)

const testOrgID = "T0TEST"

type testSource struct {
	people map[string]directory.Person
}

func (s testSource) Name() string                { return "test" }
func (s testSource) Fetch(context.Context) error { return nil }
func (s testSource) Len() int                    { return len(s.people) }
func (s testSource) Lookup(_ context.Context, id string) (directory.Person, error) {
	return s.people[id], nil
}

func newTestRouter(t *testing.T) http.Handler {
	t.Helper()
	people := []directory.Person{
		{Email: "slack@gavinmogan.com", Name: "Gavin Mogan", AvatarURL: "https://cdn.slack.com/avatar.png"},
		{Email: "noavatar@gavinmogan.com", Name: "No Avatar"},
	}
	source := testSource{people: make(map[string]directory.Person, len(people)*2)}
	for _, person := range people {
		source.people[directory.MD5Hash(person.Email)] = person
		source.people[directory.SHA256Hash(person.Email)] = person
	}
	cfg := config.New()
	cfg.AddSource(testOrgID, source)
	logger, _ := test.NewNullLogger()
	return NewRouter(&Deps{
		Logger:        logger,
		Config:        cfg,
		DefaultAvatar: []byte("<svg>default</svg>"),
	})
}

func TestHealthz(t *testing.T) {
	router := newTestRouter(t)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestResponsesPreventTransformation(t *testing.T) {
	router := newTestRouter(t)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if got, want := rec.Header().Get("Cache-Control"), "no-transform"; got != want {
		t.Errorf("Cache-Control = %q, want %q", got, want)
	}
}

func TestMetrics(t *testing.T) {
	router := newTestRouter(t)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestProfileFound(t *testing.T) {
	router := newTestRouter(t)
	hash := directory.SHA256Hash("slack@gavinmogan.com")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/slack/"+testOrgID+"/profiles/"+hash, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("CORS header = %q, want *", got)
	}
	if !strings.Contains(rec.Body.String(), "Gavin Mogan") {
		t.Errorf("body = %s, want to contain Gavin Mogan", rec.Body.String())
	}
}

func TestProfileNotFound(t *testing.T) {
	router := newTestRouter(t)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/slack/"+testOrgID+"/profiles/unknownhash", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestProfileWrongOrgID(t *testing.T) {
	router := newTestRouter(t)
	hash := directory.SHA256Hash("slack@gavinmogan.com")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/slack/WRONGORG/profiles/"+hash, nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestAvatarKnownRedirects(t *testing.T) {
	router := newTestRouter(t)
	hash := directory.MD5Hash("slack@gavinmogan.com")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/slack/"+testOrgID+"/avatar/"+hash, nil))

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	if got, want := rec.Header().Get("Location"), "https://cdn.slack.com/avatar.png"; got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
}

func TestAvatarUnknownServesDefault(t *testing.T) {
	router := newTestRouter(t)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/slack/"+testOrgID+"/avatar/unknownhash", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got, want := rec.Body.String(), "<svg>default</svg>"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
	if got, want := rec.Header().Get("Cache-Control"), "public, max-age=3600, no-transform"; got != want {
		t.Errorf("Cache-Control = %q, want %q", got, want)
	}
}

func TestAvatarUnknownWithD404(t *testing.T) {
	router := newTestRouter(t)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/slack/"+testOrgID+"/avatar/unknownhash?d=404", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestAvatarKnownPersonNoAvatarServesDefault(t *testing.T) {
	router := newTestRouter(t)
	hash := directory.SHA256Hash("noavatar@gavinmogan.com")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/slack/"+testOrgID+"/avatar/"+hash, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestCORSPreflight(t *testing.T) {
	router := newTestRouter(t)
	hash := directory.MD5Hash("slack@gavinmogan.com")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodOptions, "/slack/"+testOrgID+"/profiles/"+hash, nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}
