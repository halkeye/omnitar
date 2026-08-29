// Package directory defines the employee directory model and the Slack
// source that populates it.
package directory

import (
	"crypto/md5" //nolint:gosec // required for Gravatar-compatible hashing, not for security
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// Person is a single directory entry.
type Person struct {
	ID     string `json:"id"`
	TeamID string `json:"team_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	// AvatarURL is the upstream (Slack CDN) URL for the person's photo, if
	// known. Empty when no avatar is available.
	AvatarURL string            `json:"avatar_url,omitempty"`
	Fields    map[string]string `json:"fields,omitempty"`
}

// NormalizeEmail applies the normalization Gravatar uses before hashing:
// trim surrounding whitespace and lowercase.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// MD5Hash returns the legacy Gravatar-compatible MD5 hex digest of a
// normalized email address.
func MD5Hash(email string) string {
	sum := md5.Sum([]byte(NormalizeEmail(email))) //nolint:gosec // Gravatar compatibility, not security
	return hex.EncodeToString(sum[:])
}

// SHA256Hash returns the current Gravatar-compatible SHA256 hex digest of a
// normalized email address.
func SHA256Hash(email string) string {
	sum := sha256.Sum256([]byte(NormalizeEmail(email)))
	return hex.EncodeToString(sum[:])
}
