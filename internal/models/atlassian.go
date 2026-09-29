package models

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/ctreminiom/go-atlassian/pkg/infra/models"

	"github.com/coocood/freecache"
	"github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/store"
	freecache_store "github.com/eko/gocache/store/freecache/v4"
	"golang.org/x/oauth2"
	"golang.org/x/sync/singleflight"

	"github.com/halkeye/omnitar/internal/cachefetch"
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

	resources, err := cachefetch.Fetch[[]*accessibleResources](ctx, s.token, "https://api.atlassian.com/oauth/token/accessible-resources")
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

func (s *AtlassianSource) getIssue(ctx context.Context, siteURL, issueKey string) (*Issue, error) {

	u := s.baseURL.Clone().JoinPath("rest/api/3/issue/", issueKey)
	u.RawQuery = url.Values{"fields": []string{"key,summary,issuetype,project.key,fields.project,statusCategory,resolution,priority,status,reporter,assignee"}}.Encode()

	jiraIssue, err := cachefetch.Fetch[models.IssueSchemeV2](ctx, s.token, u.String())
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
