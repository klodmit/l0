// internal/app/run.go
package app

import (
	"context"
	"fmt"
	"l0/internal/config"
	"l0/internal/storage"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

const (
	envDev  = "dev"
	envProd = "prod"
)

func Run() {
	// Инициализация конфига
	cfg := config.MustLoad()

	// Инициализация логгера
	log := setupLogger(cfg.Env).With("service", "l0")
	log.Info("starting server")

	// root-context с отменой по сигналу
	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Подключение к базе данных
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.Db.DatabaseUser,
		cfg.Db.DatabasePass,
		cfg.Db.DatabaseHost,
		cfg.Db.DatabasePort,
		cfg.Db.DatabaseName,
	)

	pool, err := storage.InitDB(rootCtx, log, dsn)
	if err != nil {
		log.Error("storage init failed, exiting", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	repo := storage.NewOrderRepo(pool, log)

	<-rootCtx.Done()

}

func setupLogger(env string) *slog.Logger {
	switch env {
	case envDev:
		return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	case envProd:
		return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	default:
		return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}
}
