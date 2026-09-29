package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"

	"github.com/slack-go/slack"
	"golang.org/x/oauth2"

	"github.com/halkeye/omnitar/internal/cachefetch"
	"github.com/halkeye/omnitar/internal/providers"
)

var slackUserIDRegex = regexp.MustCompile(`^U[A-Z0-9]{8,}$`)

type SlackSource struct {
	slackOrgID string
	token      *Token
}

func SlackSourceOAuth2Config(clientID string, clientSource string) providers.SourceOauthContainer {
	return providers.SourceOauthContainer{
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
func NewSlackSource(slackOrgID string, token *Token) *SlackSource {
	return &SlackSource{
		slackOrgID: slackOrgID,
		token:      token,
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

type slackWrapperUserProfile struct {
	Ok          bool              `json:"ok"`
	UserProfile slack.UserProfile `json:"profile"`
}

func (s *SlackSource) getUser(ctx context.Context, slackID string, fetchDepth int) (*Person, error) {
	queryString := url.Values{"include_labels": []string{"true"}, "user": []string{slackID}}.Encode()
	slackUserProfile, err := cachefetch.Fetch[slackWrapperUserProfile](ctx, s.token, "https://slack.com/api/users.profile.get?"+queryString)
	if err != nil {
		if val, ok := errors.AsType[cachefetch.HttpError](err); ok {
			var slackErr slack.SlackErrorResponse
			if decodeErr := json.Unmarshal(val.Body, &slackErr); decodeErr != nil {
				if slackErr.Error() == "users_not_found" {
					return nil, &NotFoundError{}
				}
				err = slackErr
			}
		}
		return nil, fmt.Errorf("unable to fetch user: %w", err)
	}

	var person *Person

	if slackUserProfile.Ok == false {
		return person, &NotFoundError{}
	}

	person = &Person{
		Source: "slack",

		ID:        slackID,
		TeamID:    s.slackOrgID,
		Email:     slackUserProfile.UserProfile.Email,
		Name:      displayName(&slackUserProfile.UserProfile, slackUserProfile.UserProfile.Email),
		AvatarURL: avatarURL(&slackUserProfile.UserProfile),
		Fields:    map[string]string{},
	}

	for _, field := range slackUserProfile.UserProfile.Fields.ToMap() {
		person.Fields[field.Label] = field.Value
		if fetchDepth >= 1 && slackUserIDRegex.MatchString(field.Value) {
			parent, err := s.getUser(ctx, field.Value, fetchDepth-1)
			if err == nil {
				person.Fields[field.Label] = parent.Name
			}
		}
	}

	return person, nil
}

type slackWrapperUser struct {
	Ok   bool       `json:"ok"`
	User slack.User `json:"user"`
}

func (s *SlackSource) getUserIdByEmail(ctx context.Context, email string) (string, error) {
	queryString := url.Values{"email": []string{email}}.Encode()
	slackUser, err := cachefetch.Fetch[slackWrapperUser](ctx, s.token, "https://slack.com/api/users.lookupByEmail?"+queryString)
	if err != nil {
		if val, ok := errors.AsType[cachefetch.HttpError](err); ok {
			var slackErr slack.SlackErrorResponse
			if decodeErr := json.Unmarshal(val.Body, &slackErr); decodeErr != nil {
				if slackErr.Error() == "users_not_found" {
					return "", &NotFoundError{}
				}
				err = slackErr
			}
		}
		return "", fmt.Errorf("unable to fetch user: %w", err)
	}
	if slackUser.Ok == false {
		return "", &NotFoundError{}
	}
	return slackUser.User.ID, nil
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
