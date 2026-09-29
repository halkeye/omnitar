package cachefetch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/coocood/freecache"
	"github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/store"
	freecache_store "github.com/eko/gocache/store/freecache/v4"
	"golang.org/x/oauth2"
	"golang.org/x/sync/singleflight"

	"github.com/halkeye/omnitar/internal/logger"
	"github.com/halkeye/omnitar/internal/providers"
)

var sg singleflight.Group
var cacheManager *cache.Cache[[]byte]

func init() {
	freecacheStore := freecache_store.NewFreecache(freecache.NewCache(1024*10000), store.WithExpiration(30*time.Minute))
	cacheManager = cache.New[[]byte](freecacheStore)
}

type TokenUpdater interface {
	OAuthConfig(ctx context.Context) *providers.SourceOauthContainer
	GetAccessToken() string
	AsOAuthToken() *oauth2.Token
	UpdateFromOauth(ctx context.Context, newToken *oauth2.Token) error
}

type HttpError struct {
	Url        string
	StatusCode int
	Body       []byte
}

func (he HttpError) Error() string {
	return fmt.Sprintf("http error - %s - %d", he.Url, he.StatusCode)
}

func Fetch[T any](ctx context.Context, token TokenUpdater, url string) (T, error) {
	val, err, _ := sg.Do(url, func() (any, error) {
		cacheVal, err := cacheManager.Get(ctx, url)
		if err != nil && !(store.NotFound{}).Is(err) {
			return nil, fmt.Errorf("cache get: %w", err)
		}

		if cacheVal != nil {
			return cacheVal, nil
		}

		oauth2Config := token.OAuthConfig(ctx)
		tokenSource := oauth2Config.Config.TokenSource(ctx, token.AsOAuthToken())
		newToken, err := tokenSource.Token()
		if err != nil {
			return nil, fmt.Errorf("failed to refresh token: %w", err)
		}

		if newToken.AccessToken != token.GetAccessToken() {
			err := token.UpdateFromOauth(ctx, newToken)
			if err != nil {
				return nil, fmt.Errorf("failed to update token in database: %w", err)
			}
		}
		client := oauth2.NewClient(ctx, tokenSource)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to make request: %w", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to do request: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("unable to read body: %w", err)
		}

		if resp.StatusCode != 200 {
			return nil, &HttpError{
				StatusCode: resp.StatusCode,
				Url:        url,
				Body:       body,
			}
		}

		err = cacheManager.Set(ctx, url, body, store.WithExpiration(time.Hour))
		if err != nil {
			logger.FromContext(ctx).WithField("url", url).WithField("size", len(cacheVal)).WithError(err).Error("cache set")
		}

		return body, nil
	})
	var target T

	if err != nil {
		return target, err
	}

	err = json.Unmarshal(val.([]byte), &target)
	if err != nil {
		return target, fmt.Errorf("unable to decode body into target")
	}

	return target, err
}
