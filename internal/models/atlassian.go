package models

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/ctreminiom/go-atlassian/pkg/infra/models"
	"gorm.io/gorm"

	"github.com/coocood/freecache"
	"github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/store"
	freecache_store "github.com/eko/gocache/store/freecache/v4"
	clone "github.com/huandu/go-clone/generic"
	"golang.org/x/oauth2"
	"golang.org/x/sync/singleflight"

	"github.com/halkeye/omnitar/internal/database"
	"github.com/halkeye/omnitar/internal/logger"
)

type AtlassianSource struct {
	token        *Token
	baseURL      *url.URL
	cacheManager *cache.Cache[[]byte]
	sg           singleflight.Group
}

func AtlassianSourceOAuth2Config(clientID string, clientSource string) SourceOauthContainer {
	return SourceOauthContainer{
		Options: []oauth2.AuthCodeOption{
			oauth2.SetAuthURLParam("access_type", "offline_access"),
			oauth2.SetAuthURLParam("audience", "api.atlassian.com"),
		},
		Config: oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSource,
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://auth.atlassian.com/authorize",
				TokenURL: "https://auth.atlassian.com/oauth/token",
				// https://api.atlassian.com/oauth/token/accessible-resources
			},
			Scopes: []string{
				// Granular scopes
				// "read:issue-meta:jira", "read:issue-security-level:jira", "read:issue.vote:jira", "read:issue.changelog:jira", "read:avatar:jira", "read:issue:jira", "read:status:jira", "read:user:jira", "read:field-configuration:jira",
				// Classic scopes
				"read:jira-work", "read:jira-user", "offline_access",
			},
		},
	}
}

var _ IssueSource = (*AtlassianSource)(nil)

// NewAtlassianSource returns a Source backed by the given Atlassian bot token.
func NewAtlassianSource(cloudID string, token *Token) *AtlassianSource {
	freecacheStore := freecache_store.NewFreecache(freecache.NewCache(1024*1000), store.WithExpiration(10*time.Second))

	u, err := url.Parse(fmt.Sprintf("https://api.atlassian.com/ex/jira/%s/", cloudID))
	if err != nil {
		// shouldn't happen because its coming from atlassian sdks
		panic(err)
	}

	return &AtlassianSource{
		token:        token,
		baseURL:      u,
		cacheManager: cache.New[[]byte](freecacheStore),
	}
}

func (s *AtlassianSource) Name() string { return "atlassian" }

func (s *AtlassianSource) LookupIssue(ctx context.Context, issue string) (*Issue, error) {
	person, err := s.getIssue(ctx, issue)
	if err != nil {
		return nil, err
	}
	return person, nil
}

func (s *AtlassianSource) getIssue(ctx context.Context, issueKey string) (*Issue, error) {
	val, err, _ := s.sg.Do(issueKey, func() (any, error) {
		var person *Issue
		cacheVal, err := s.cacheManager.Get(ctx, issueKey)
		if err != nil && !(store.NotFound{}).Is(err) {
			return nil, fmt.Errorf("cache get: %w", err)
		}

		if cacheVal != nil {
			if len(cacheVal) == 0 {
				return person, &NotFoundError{}
			}
			err = json.Unmarshal(cacheVal, &person)
			if err != nil {
				return nil, fmt.Errorf("cache unmarshal: %w", err)
			}
			return person, nil
		}

		u := s.baseURL.Clone().JoinPath("rest/api/3/issue/", issueKey)
		u.RawQuery = url.Values{"fields": []string{"key,summary,issuetype,project.key,fields.project,statusCategory,resolution,priority,status,reporter,assignee"}}.Encode()

		oauth2Config := AtlassianSourceOAuth2Config("", "")
		tokenSource := oauth2Config.Config.TokenSource(ctx, s.token.AsOAuthToken())
		newToken, err := tokenSource.Token()
		if err != nil {
			return nil, fmt.Errorf("failed to refresh token: %w", err)
		}
		if newToken.AccessToken != s.token.AccessToken {
			db := database.FromContext(ctx)
			_, err := gorm.G[Token](db).Where("id = ?", s.token.ID).Updates(ctx, Token{
				ID:           s.token.ID,
				AccessToken:  newToken.AccessToken,
				RefreshToken: newToken.RefreshToken,
				ExpiresAt:    newToken.Expiry,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to update token in database: %w", err)
			}
		}
		client := oauth2.NewClient(ctx, tokenSource)
		req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to make request: %w", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to do request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("bad request to atlassian - %s - %d", u.String(), resp.StatusCode)
		}

		var issue models.IssueSchemeV2
		err = json.NewDecoder(resp.Body).Decode(&issue)
		if err != nil {
			return nil, fmt.Errorf("unable to decode issue")
		}

		person = &Issue{
			Title:    issue.Fields.Summary,
			Key:      issue.Key,
			Type:     issue.Fields.IssueType.Name,
			State:    issue.Fields.Status.Name,
			Priority: issue.Fields.Priority.Name,
			Fields:   map[string]string{},
		}

		if issue.Fields.Reporter != nil {
			person.Reporter = issue.Fields.Reporter.DisplayName
		}

		if issue.Fields.Assignee != nil {
			person.Assignee = issue.Fields.Assignee.DisplayName
		}

		cacheVal, err = json.Marshal(person)
		if err != nil {
			return nil, fmt.Errorf("cache marshal: %w", err)
		}

		err = s.cacheManager.Set(ctx, issue, cacheVal, store.WithExpiration(time.Hour))
		if err != nil {
			logger.FromContext(ctx).WithField("issue", issue).WithField("size", len(cacheVal)).WithError(err).Warn("cache set")
		}
		return person, nil
	})
	if err != nil {
		return nil, err
	}
	return new(clone.Clone[Issue](*(val.(*Issue)))), nil
}
