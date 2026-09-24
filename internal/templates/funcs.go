package templates

import (
	"context"
)

func SetBaseURL(ctx context.Context, baseURL string) context.Context {
	return context.WithValue(ctx, BaseURLKey, baseURL)
}

func BaseURL(ctx context.Context) string {
	if baseURL, ok := ctx.Value(BaseURLKey).(string); ok {
		return baseURL
	}
	return ""
}

func Flashes(ctx context.Context) []string {
	if flashes, ok := ctx.Value(FlashesKey).([]string); ok {
		return flashes
	}
	return []string{}
}

func AddFlash(ctx context.Context, flash ...string) context.Context {
	flashes := Flashes(ctx)
	return context.WithValue(ctx, FlashesKey, append(flashes, flash...))
}
