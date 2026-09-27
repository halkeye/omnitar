package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/coocood/freecache"
	"github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/store"
	freecache_store "github.com/eko/gocache/store/freecache/v4"
	clone "github.com/huandu/go-clone/generic"
	"github.com/slack-go/slack"
	"golang.org/x/oauth2"
	"golang.org/x/sync/singleflight"

	"github.com/halkeye/omnitar/internal/logger"
)

var slackUserIDRegex = regexp.MustCompile(`^U[A-Z0-9]{8,}$`)

type SlackSource struct {
	client       *slack.Client
	slackOrgID   string
	cacheManager *cache.Cache[[]byte]
	sg           singleflight.Group
}

func SlackSourceOAuth2Config(clientID string, clientSource string) SourceOauthContainer {
	return SourceOauthContainer{
		Config: oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSource,
			Endpoint: oauth2.Endpoint{
				AuthURL:   "https://slack.com/oauth/v2/authorize",
				TokenURL:  "https://slack.com/api/oauth.v2.access",
				AuthStyle: oauth2.AuthStyleInParams,
			},
			Scopes: []string{"users.profile:read", "users:read", "users:read.email"},
		},
		Options: []oauth2.AuthCodeOption{},
	}
}

// NewSlackSource returns a Source backed by the given Slack bot token.
func NewSlackSource(slackOrgID string, token *oauth2.Token) *SlackSource {
	freecacheStore := freecache_store.NewFreecache(freecache.NewCache(1024*1000), store.WithExpiration(10*time.Second))

	return &SlackSource{
		slackOrgID:   slackOrgID,
		client:       slack.New(token.AccessToken),
		cacheManager: cache.New[[]byte](freecacheStore),
	}
}

var _ PersonSource = (*SlackSource)(nil)

// Name implements Source.
func (s *SlackSource) Name() string { return "slack" }

func (s *SlackSource) LookupPerson(ctx context.Context, email string) (*Person, error) {
	slackID, err := s.getUserIdByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	person, err := s.getUser(ctx, slackID, 1)
	if err != nil {
		return nil, err
	}
	return person, nil
}

func (s *SlackSource) getUser(ctx context.Context, slackID string, fetchDepth int) (*Person, error) {
	val, err, _ := s.sg.Do(slackID, func() (any, error) {
		var person *Person
		cacheVal, err := s.cacheManager.Get(ctx, slackID)
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

		slackUserProfile, err := s.client.GetUserProfile(&slack.GetUserProfileParameters{UserID: slackID, IncludeLabels: true})
		if err != nil {
			if slackErr, ok := errors.AsType[slack.SlackErrorResponse](err); ok {
				if slackErr.Error() == "users_not_found" {
					s.cacheManager.Set(ctx, slackID, []byte{}, store.WithExpiration(time.Hour))
					return person, &NotFoundError{}
				}
			}
			return nil, fmt.Errorf("slack users.profile.get: %w", err)
		}

		person = &Person{
			ID:        slackID,
			TeamID:    s.slackOrgID,
			Email:     slackUserProfile.Email,
			Name:      displayName(slackUserProfile, slackUserProfile.Email),
			AvatarURL: avatarURL(slackUserProfile),
			Fields:    map[string]string{},
		}

		for _, field := range slackUserProfile.Fields.ToMap() {
			person.Fields[field.Label] = field.Value
			if fetchDepth >= 1 && slackUserIDRegex.MatchString(field.Value) {
				parent, err := s.getUser(ctx, field.Value, fetchDepth-1)
				if err == nil {
					person.Fields[field.Label] = parent.Name
				}
			}
		}

		cacheVal, err = json.Marshal(person)
		if err != nil {
			return nil, fmt.Errorf("cache marshal: %w", err)
		}
		err = s.cacheManager.Set(ctx, slackID, cacheVal, store.WithExpiration(time.Hour))
		if err != nil {
			logger.FromContext(ctx).WithField("slackID", slackID).WithField("size", len(cacheVal)).WithError(err).Warn("cache set")
		}
		return person, nil
	})
	if err != nil {
		return nil, err
	}
	return new(clone.Clone[Person](*(val.(*Person)))), nil
}

func (s *SlackSource) getUserIdByEmail(ctx context.Context, email string) (string, error) {
	cacheKey := fmt.Sprintf("email-to-slackid:%s", email)
	val, err, _ := s.sg.Do(cacheKey, func() (any, error) {
		cacheVal, err := s.cacheManager.Get(ctx, cacheKey)
		if err != nil && !(store.NotFound{}).Is(err) {
			return nil, fmt.Errorf("cache get: %w", err)
		}

		if cacheVal != nil {
			return string(cacheVal), nil
		}

		slackUser, err := s.client.GetUserByEmailContext(ctx, email)
		if err != nil {
			if slackErr, ok := errors.AsType[slack.SlackErrorResponse](err); ok {
				if slackErr.Error() == "users_not_found" {
					s.cacheManager.Set(ctx, cacheKey, []byte{}, store.WithExpiration(time.Hour))
					return nil, &NotFoundError{}
				}
			}
			return nil, fmt.Errorf("slack users.profile.get: %w", err)
		}

		err = s.cacheManager.Set(ctx, cacheKey, []byte(slackUser.ID), store.WithExpiration(time.Hour))
		if err != nil {
			logger.FromContext(ctx).WithField("slackID", slackUser.ID).WithError(err).Error("cache set failed")
		}
		return slackUser.ID, nil
	})
	if err != nil {
		return "", err
	}
	return val.(string), nil
}

func displayName(u *slack.UserProfile, email string) string {
	switch {
	case u.RealName != "":
		return u.RealName
	case u.DisplayName != "":
		return u.DisplayName
	default:
		return email
	}
}

func avatarURL(u *slack.UserProfile) string {
	switch {
	case u.ImageOriginal != "":
		return u.ImageOriginal
	case u.Image512 != "":
		return u.Image512
	default:
		return u.Image192
	}
}
