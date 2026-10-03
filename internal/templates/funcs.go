package templates

import (
	"context"
	"html"
)

func IsDev(ctx context.Context) bool {
	return ctx.Value(IsDevKey) == true
}

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

func copyPaste(text string) string {
	return text + "<br /><copy-paste>" + html.EscapeString(text) + "</copy-paste>"
}

func tokenDeleteConfirm(isLast bool) string {
	if isLast {
		return "Deleting your last token will log you out and your account will not be recoverable. Really delete?"
	}
	return "really delete?"
}
