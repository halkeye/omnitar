package models

type OAuthProvider string

const (
	Slack     OAuthProvider = "slack"
	Atlassian OAuthProvider = "atlassian"
)
