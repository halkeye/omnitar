package api

import _ "embed"

//go:embed static/default-avatar.svg
var defaultAvatarSVG []byte

// DefaultAvatarSVG returns the bundled fallback avatar image served when a
// profileIdentifier is unknown or has no photo on file.
func DefaultAvatarSVG() []byte {
	return defaultAvatarSVG
}
