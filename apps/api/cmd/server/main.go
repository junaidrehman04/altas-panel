package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/junaidrehman04/altas/apps/api/internal/config"
	"github.com/junaidrehman04/altas/apps/api/internal/db"
	"github.com/junaidrehman04/altas/apps/api/internal/httpapi"
)

func main() {
	cfg := config.Load()
	logger := log.New(os.Stdout, "", log.LstdFlags)

	sqlDB, err := db.Open(cfg.DBPath)
	if err != nil {
		logger.Fatalf("database: %v", err)
	}
	defer sqlDB.Close()

	api := httpapi.New(cfg, sqlDB, logger)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go api.RunMaintenance(ctx)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           api,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       20 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()

	logger.Printf("altas api listening on %s (%s)", cfg.Addr, cfg.Env)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Fatalf("server: %v", err)
	}
}
