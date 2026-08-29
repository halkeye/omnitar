package directory

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/coocood/freecache"
	"github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/store"
	freecache_store "github.com/eko/gocache/store/freecache/v4"
	clone "github.com/huandu/go-clone/generic"
	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"golang.org/x/sync/singleflight"
)

var slackUserIDRegex = regexp.MustCompile(`^U[A-Z0-9]{8,}$`)

type SlackSource struct {
	client       *slack.Client
	emailCounter map[string]struct{}
	slackOrgID   string
	logger       *logrus.Logger
	cacheManager *cache.Cache[[]byte]
	emailMap     *sync.Map
	sg           singleflight.Group
}

// NewSlackSource returns a Source backed by the given Slack bot token.
func NewSlackSource(logger *logrus.Logger, slackOrgID string, token string) *SlackSource {
	freecacheStore := freecache_store.NewFreecache(freecache.NewCache(1024*1000), store.WithExpiration(10*time.Second))

	return &SlackSource{
		logger:       logger,
		emailCounter: map[string]struct{}{},
		slackOrgID:   slackOrgID,
		emailMap:     new(sync.Map),
		client:       slack.New(token),
		cacheManager: cache.New[[]byte](freecacheStore),
	}
}

// Name implements Source.
func (s *SlackSource) Name() string { return "slack" }

// Fetch implements Source, returning every non-deleted, non-bot Slack member
// that has an email address on file.
func (s *SlackSource) Fetch(ctx context.Context) error {
	users, err := s.client.GetUsersContext(ctx)
	if err != nil {
		return fmt.Errorf("slack users.list: %w", err)
	}

	for _, u := range users {
		if u.Deleted || u.IsBot || u.Profile.Email == "" {
			continue
		}
		s.emailCounter[u.Profile.Email] = struct{}{}
		s.emailMap.Store(MD5Hash(u.Profile.Email), u.ID)
		s.emailMap.Store(SHA256Hash(u.Profile.Email), u.ID)
	}
	return nil
}

func (s *SlackSource) getUser(ctx context.Context, slackID string, fetchDepth int) (Person, error) {
	val, err, _ := s.sg.Do(slackID, func() (any, error) {
		var person *Person
		cacheVal, err := s.cacheManager.Get(ctx, slackID)
		s.logger.WithField("slackID", slackID).WithError(err).Debug("cache get")
		if err != nil && !(store.NotFound{}).Is(err) {
			return nil, fmt.Errorf("cache get: %w", err)
		}
		if cacheVal != nil {
			err = json.Unmarshal(cacheVal, &person)
			if err != nil {
				return nil, fmt.Errorf("cache unmarshal: %w", err)
			}
			return person, nil
		}

		s.logger.WithField("slackID", slackID).Debug("cache miss")
		slackUserProfile, err := s.client.GetUserProfile(&slack.GetUserProfileParameters{UserID: slackID, IncludeLabels: true})
		if err != nil {
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
		s.logger.WithField("slackID", slackID).WithField("size", len(cacheVal)).WithError(err).Debug("cache set")
		if err != nil {
			s.logger.WithField("slackID", slackID).WithField("size", len(cacheVal)).WithError(err).Warn("cache set")
		}
		return person, nil
	})
	if err != nil {
		return Person{}, err
	}
	return clone.Clone[Person](*(val.(*Person))), nil
}

func (s *SlackSource) Lookup(ctx context.Context, hashedToken string) (Person, error) {
	val, ok := s.emailMap.Load(hashedToken)
	if !ok {
		s.logger.WithField("hashedToken", hashedToken).Debug("unknown token to email")
		return Person{}, nil
	}

	slackID := val.(string)

	person, err := s.getUser(ctx, slackID, 1)
	if err != nil {
		s.logger.WithError(err).Debug("err")
		return Person{}, err
	}
	s.logger.WithField("person", val).Debug("person")
	return person, nil
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

func (s *SlackSource) Len() int {
	return len(s.emailCounter)
}
