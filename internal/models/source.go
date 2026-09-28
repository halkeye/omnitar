package models

import (
	"context"

	"golang.org/x/oauth2"
)

var _ error = &NotFoundError{}

type NotFoundError struct{}

func (e *NotFoundError) Error() string {
	return "not found"
}

type IssueSource interface {
	LookupIssue(ctx context.Context, key string) (*Issue, error)
}

type PersonSource interface {
	LookupPerson(ctx context.Context, key string) (*Person, error)
}

type SourceOauthContainer struct {
	Config  oauth2.Config
	Options []oauth2.AuthCodeOption
}
