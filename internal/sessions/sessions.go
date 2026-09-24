// Package sessions wraps github.com/gin-contrib/sessions to provide
// generic, typed Get/Set helpers and a MustSave convenience method.
package sessions

import (
	"encoding/json"
	"fmt"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// Session wraps a gin-contrib/sessions.Session, exposing the same
// untyped API plus the generic Get/Set helpers and MustSave below.
type Session struct {
	sessions.Session
}

// Store and Options are re-exported so callers never need to import
// github.com/gin-contrib/sessions directly.
type (
	Store   = sessions.Store
	Options = sessions.Options
)

// DefaultKey is re-exported from github.com/gin-contrib/sessions.
const DefaultKey = sessions.DefaultKey

// Default returns the session for the current request, wrapped for generic access.
func Default(c *gin.Context) Session {
	return Session{sessions.Default(c)}
}

// DefaultMany returns the named session for the current request, wrapped for generic access.
func DefaultMany(c *gin.Context, name string) Session {
	return Session{sessions.DefaultMany(c, name)}
}

// Sessions returns a gin middleware that stores the session under the given name.
func Sessions(name string, store Store) gin.HandlerFunc {
	return sessions.Sessions(name, store)
}

// SessionsMany returns a gin middleware that stores sessions under the given names.
func SessionsMany(names []string, store Store) gin.HandlerFunc {
	return sessions.SessionsMany(names, store)
}

// Get retrieves a typed value from the session, returning the zero value of T
// if the key is missing or the stored value is not of type T.
func Get[T any](s Session, key string) T {
	var ret T
	b, ok := s.Session.Get(key).([]byte)
	if !ok {
		return ret
	}
	if len(b) == 0 {
		return ret
	}
	err := json.Unmarshal(b, &ret)
	if err != nil {
		panic(fmt.Errorf("unable to unmarshal: %w", err))
	}
	return ret
}

// Set stores a typed value in the session.
func Set(s Session, key string, val any) {
	b, err := json.Marshal(val)
	if err != nil {
		panic(fmt.Errorf("unable to marshal for set: %w", err))
	}
	s.Session.Set(key, b)
}

// MustSave saves the session, panicking if the save fails.
func MustSave(s Session) {
	if err := s.Session.Save(); err != nil {
		panic(err)
	}
}
