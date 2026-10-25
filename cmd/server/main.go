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

	root "github.com/halkeye/omnitar"
	"github.com/halkeye/omnitar/internal/config"
	api "github.com/halkeye/omnitar/internal/http"
	"github.com/halkeye/omnitar/internal/logger"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		panic(fmt.Errorf("failed to load config: %w", err))
	}

	var staticHandler http.Handler
	if cfg.IsDev() {
		staticHandler, err = createViteProxy("http://localhost:5173")
		if err != nil {
			logger.Default.Error("failed to create Vite proxy", "error", err)
			os.Exit(1)
		}
	} else {
		serverRoot, err := fs.Sub(root.StaticFiles, "static")
		if err != nil {
			logger.Default.Error("chdir to static", "error", err)
			os.Exit(1)
		}
		staticHandler = http.FileServerFS(serverRoot)
	}

	router := api.New(api.WithLogger(logger.Default), api.WithConfig(cfg), api.WithStaticHandler(staticHandler))

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Default.Info("listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Default.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Default.Info("shutting down")

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Default.Error("graceful shutdown failed", "error", err)
	}
}

func createViteProxy(target string) (http.Handler, error) {
	url, err := url.Parse(target)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Vite server URL: %w", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(url)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Update the request to point to the Vite server
		r.Host = url.Host
		r.URL.Scheme = url.Scheme
		r.URL.Host = url.Host
		proxy.ServeHTTP(w, r)
	}), nil
}
