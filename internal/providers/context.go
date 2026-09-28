package providers

import (
	"context"

	"golang.org/x/oauth2"
)

type OAuthProvider string

const (
	Slack     OAuthProvider = "slack"
	Atlassian OAuthProvider = "atlassian"
)

type SourceOauthContainer struct {
	Config  oauth2.Config
	Options []oauth2.AuthCodeOption
}

type providers struct {
	providers map[OAuthProvider]SourceOauthContainer
}

// Define the context key type.
type contextKey string

var providersKey contextKey = "providers"

func WithValue(ctx context.Context, provider *providers) context.Context {
	return context.WithValue(ctx, providersKey, provider)
}

func FromContext(ctx context.Context) *providers {
	val, ok := ctx.Value(providersKey).(*providers)
	if !ok {
		panic("no provider map available")
	}
	return val
}

func New() *providers {
	return &providers{
		providers: make(map[OAuthProvider]SourceOauthContainer),
	}
}

func (p *providers) Register(provider OAuthProvider, container SourceOauthContainer) {
	p.providers[provider] = container
}

func (p *providers) Get(provider OAuthProvider) *SourceOauthContainer {
	val, ok := p.providers[provider]
	if !ok {
		return nil
	}
	return new(val)
}
