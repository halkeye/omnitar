// Command profile-http serves a Gravatar-compatible profile/avatar API and
// hover-card web component backed by Slack workspaces
// directory.
package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	root "github.com/halkeye/omnitar"
	"github.com/halkeye/omnitar/internal/config"
	api "github.com/halkeye/omnitar/internal/http"
)

func main() {

	cfg, err := config.Load()
	if err != nil {
		panic(fmt.Errorf("failed to load config: %w", err))
	}

	var staticHandler http.Handler
	if cfg.IsDev() {
		staticHandler = createViteProxy(cfg.Logger(), "http://localhost:5173")
	} else {
		serverRoot, err := fs.Sub(root.StaticFiles, "static")
		if err != nil {
			cfg.Logger().WithError(err).Fatal("chdir to static")
		}
		staticHandler = http.FileServerFS(serverRoot)
	}

	router := api.NewRouter(&api.Deps{
		Logger:        cfg.Logger(),
		Config:        cfg,
		DefaultAvatar: api.DefaultAvatarSVG(),
		StaticHandler: staticHandler,
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		cfg.Logger().WithField("port", cfg.Port).Info("listening")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			cfg.Logger().WithError(err).Fatal("server failed")
		}
	}()

	<-ctx.Done()
	cfg.Logger().Info("shutting down")

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		cfg.Logger().WithError(err).Error("graceful shutdown failed")
	}
}

func createViteProxy(logger *logrus.Logger, target string) http.Handler {
	url, err := url.Parse(target)
	if err != nil {
		logger.Fatal(err)
	}
	proxy := httputil.NewSingleHostReverseProxy(url)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Update the request to point to the Vite server
		r.Host = url.Host
		r.URL.Scheme = url.Scheme
		r.URL.Host = url.Host
		proxy.ServeHTTP(w, r)
	})
}
