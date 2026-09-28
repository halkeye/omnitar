package models

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/ctreminiom/go-atlassian/pkg/infra/models"
	"gorm.io/gorm"

	"github.com/coocood/freecache"
	"github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/store"
	freecache_store "github.com/eko/gocache/store/freecache/v4"
	"golang.org/x/oauth2"
	"golang.org/x/sync/singleflight"

	"github.com/halkeye/omnitar/internal/database"
	"github.com/halkeye/omnitar/internal/logger"
	"github.com/halkeye/omnitar/internal/providers"
)

type AtlassianSource struct {
	cloudID      string
	token        *Token
	baseURL      *url.URL
	cacheManager *cache.Cache[[]byte]
	sg           singleflight.Group
}

func AtlassianSourceOAuth2Config(clientID string, clientSource string) providers.SourceOauthContainer {
	return providers.SourceOauthContainer{
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
		cloudID:      cloudID,
		token:        token,
		baseURL:      u,
		cacheManager: cache.New[[]byte](freecacheStore),
	}
}

func (s *AtlassianSource) Name() string { return "atlassian" }

func (s *AtlassianSource) LookupIssue(ctx context.Context, issueKey string) (*Issue, error) {
	type accessibleResources struct {
		ID   string `json:"id"`
		URL  string `json:"url"`
		Name string `json:"name"`
	}

	resources := []*accessibleResources{}
	err := s.fetch(ctx, "https://api.atlassian.com/oauth/token/accessible-resources", &resources)
	if err != nil {
		return nil, err
	}

	idx := slices.IndexFunc(resources, func(r *accessibleResources) bool {
		return r.ID == s.cloudID
	})

	if idx == -1 {
		return nil, &NotFoundError{}
	}

	issue, err := s.getIssue(ctx, resources[idx].URL, issueKey)
	if err != nil {
		return nil, err
	}
	return issue, nil
}

func (s *AtlassianSource) fetch(ctx context.Context, url string, target any) error {
	val, err, _ := s.sg.Do(url, func() (any, error) {
		cacheVal, err := s.cacheManager.Get(ctx, url)
		if err != nil && !(store.NotFound{}).Is(err) {
			return nil, fmt.Errorf("cache get: %w", err)
		}

		if cacheVal != nil {
			return cacheVal, nil
		}

		oauth2Config := providers.FromContext(ctx).Get(providers.Atlassian)
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
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to make request: %w", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to do request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("bad request to atlassian - %s - %d", url, resp.StatusCode)
		}

		err = json.NewDecoder(resp.Body).Decode(&target)
		if err != nil {
			return nil, fmt.Errorf("unable to decode body into target")
		}

		err = s.cacheManager.Set(ctx, url, cacheVal, store.WithExpiration(time.Hour))
		if err != nil {
			logger.FromContext(ctx).WithField("url", url).WithField("size", len(cacheVal)).WithError(err).Error("cache set")
		}

		return nil, nil
	})
	if err != nil {
		target = val
	}
	return err
}

func (s *AtlassianSource) getIssue(ctx context.Context, siteURL, issueKey string) (*Issue, error) {

	u := s.baseURL.Clone().JoinPath("rest/api/3/issue/", issueKey)
	u.RawQuery = url.Values{"fields": []string{"key,summary,issuetype,project.key,fields.project,statusCategory,resolution,priority,status,reporter,assignee"}}.Encode()

	var jiraIssue models.IssueSchemeV2
	err := s.fetch(ctx, u.String(), &jiraIssue)
	if err != nil {
		return nil, err
	}

	issue := &Issue{
		Source: "jira",

		Title:          jiraIssue.Fields.Summary,
		URL:            strings.TrimSuffix(siteURL, "/") + "/browse/" + jiraIssue.Key,
		Key:            jiraIssue.Key,
		IssueType:      jiraIssue.Fields.IssueType.Name,
		IssueTypeIcon:  strings.ReplaceAll(jiraIssue.Fields.IssueType.IconURL, strings.TrimSuffix(s.baseURL.String(), "/"), strings.TrimSuffix(siteURL, "/")),
		Status:         jiraIssue.Fields.Status.Name,
		StatusCategory: jiraIssue.Fields.Status.StatusCategory.Name,
		StatusColor:    jiraIssue.Fields.Status.StatusCategory.ColorName,
		Priority:       jiraIssue.Fields.Priority.Name,
		PriorityIcon:   strings.ReplaceAll(jiraIssue.Fields.Priority.IconURL, strings.TrimSuffix(s.baseURL.String(), "/"), strings.TrimSuffix(siteURL, "/")),
		Fields:         map[string]string{},
	}

	if jiraIssue.Fields.Reporter != nil {
		issue.Reporter = jiraIssue.Fields.Reporter.DisplayName
		issue.ReporterIcon = strings.ReplaceAll(jiraIssue.Fields.Reporter.AvatarUrls.Four8X48, strings.TrimSuffix(s.baseURL.String(), "/"), strings.TrimSuffix(siteURL, "/"))
		issue.ReporterEmail = jiraIssue.Fields.Reporter.EmailAddress
	}

	if jiraIssue.Fields.Assignee != nil {
		issue.Assignee = jiraIssue.Fields.Assignee.DisplayName
		issue.AssigneeIcon = strings.ReplaceAll(jiraIssue.Fields.Assignee.AvatarUrls.Four8X48, strings.TrimSuffix(s.baseURL.String(), "/"), strings.TrimSuffix(siteURL, "/"))
		issue.AssigneeEmail = jiraIssue.Fields.Assignee.EmailAddress
	}

	return issue, nil
}
