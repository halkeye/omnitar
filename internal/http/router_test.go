package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

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

// notFoundStaticHandler stands in for the real webcomponent asset handler in
// tests that don't exercise it, so NoRoute doesn't panic on a nil handler.
var notFoundStaticHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNotFound)
})

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
	cfg.Database_, _ = gorm.Open(sqlite.Open("file::memory:?cache=shared"))
	if err := cfg.SetupDB(t.Context()); err != nil {
		t.Fatalf("SetupDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := cfg.Close(); err != nil {
			t.Errorf("cfg.Close() error = %v", err)
		}
	})
	if err := cfg.AddSource(testOrgID, source); err != nil {
		t.Fatalf("AddSource() error = %v", err)
	}
	logger, _ := test.NewNullLogger()
	return NewRouter(&Deps{
		Logger:        logger,
		Config:        cfg,
		DefaultAvatar: []byte("<svg>default</svg>"),
		StaticHandler: notFoundStaticHandler,
	})
}

// newSlackAuthTestRouter builds a router with Slack OAuth credentials
// configured, so /slack/auth is registered. apiURL, when non-empty,
// overrides the Slack API base URL the OAuth exchange hits. It also returns
// the underlying config, so tests can inspect the sources it registers.
func newSlackAuthTestRouter(t *testing.T, apiURL string) (http.Handler, config.Config) {
	t.Helper()
	cfg := config.New()
	cfg.SlackClientID_ = "test-client-id"
	cfg.SlackClientSecret_ = "test-client-secret"
	cfg.Database_, _ = gorm.Open(sqlite.Open("file::memory:?cache=shared"))
	if err := cfg.SetupDB(t.Context()); err != nil {
		t.Fatalf("SetupDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := cfg.Close(); err != nil {
			t.Errorf("cfg.Close() error = %v", err)
		}
	})
	logger, _ := test.NewNullLogger()
	router := NewRouter(&Deps{
		Logger:        logger,
		Config:        cfg,
		SlackAPIURL:   apiURL,
		StaticHandler: notFoundStaticHandler,
	})
	return router, cfg
}

func TestSlackAuthNotRegisteredWithoutCredentials(t *testing.T) {
	router := newTestRouter(t)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/slack/auth", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestSlackAuthErrorParam(t *testing.T) {
	router, _ := newSlackAuthTestRouter(t, "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/slack/auth?error=access_denied", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got, want := rec.Body.String(), "error installing app"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

func TestSlackAuthMissingCode(t *testing.T) {
	router, _ := newSlackAuthTestRouter(t, "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/slack/auth", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "code") {
		t.Errorf("body = %q, want it to mention the missing 'code' param", rec.Body.String())
	}
}

func TestSlackAuthExchangeFailure(t *testing.T) {
	slackAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"ok":false,"error":"invalid_code"}`)
	}))
	defer slackAPI.Close()

	router, _ := newSlackAuthTestRouter(t, slackAPI.URL+"/")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/slack/auth?code=badcode", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
}

func TestSlackAuthSuccess(t *testing.T) {
	slackAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"ok":true,"access_token":"xoxb-test","team":{"id":"T0INSTALL","name":"Test Team"}}`)
	}))
	defer slackAPI.Close()

	router, _ := newSlackAuthTestRouter(t, slackAPI.URL+"/")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/slack/auth?code=goodcode", nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusFound, rec.Body.String())
	}
	if got, want := rec.Header().Get("Location"), "/"; got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
}

// TestSlackAuthSameTeamTwice simulates a team re-installing (or
// re-authorizing) the app: the OAuth callback fires twice for the same
// team ID. The second AddSource call must replace the first source/
// refresher rather than erroring out or leaking the old one.
func TestSlackAuthSameTeamTwice(t *testing.T) {
	const teamID = "T0INSTALL"
	tokens := []string{"xoxb-first", "xoxb-second"}
	var requestCount int
	slackAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		token := tokens[requestCount]
		requestCount++
		fmt.Fprintf(w, `{"ok":true,"access_token":%q,"team":{"id":%q,"name":"Test Team"}}`, token, teamID)
	}))
	defer slackAPI.Close()

	router, cfg := newSlackAuthTestRouter(t, slackAPI.URL+"/")

	for i, code := range []string{"code1", "code2"} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/slack/auth?code="+code, nil))
		if rec.Code != http.StatusFound {
			t.Fatalf("auth #%d: status = %d, want %d, body=%s", i+1, rec.Code, http.StatusFound, rec.Body.String())
		}
	}

	if requestCount != 2 {
		t.Fatalf("Slack API calls = %d, want 2", requestCount)
	}
	if source := cfg.Source(teamID); source == nil {
		t.Fatalf("Source(%q) = nil, want the replaced source to still be registered", teamID)
	}
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
