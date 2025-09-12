package storage

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func InitDB(ctx context.Context, log *slog.Logger, dsn string) (*pgxpool.Pool, error) {
	// Парсим строку подключения и добавляем в конфиг
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}

	cfg.MaxConns = 10
	cfg.MinConns = 2
	cfg.HealthCheckPeriod = 30 * time.Second
	cfg.ConnConfig.ConnectTimeout = 5 * time.Second

	connectCtx := context.Background()

	// Инициализируем пул соединений
	pool, err := pgxpool.NewWithConfig(connectCtx, cfg)
	if err != nil {
		return nil, err
	}

	// Быстрый пинг базы
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, err
	}

	log.Info("db connected", "user", cfg.ConnConfig.User, "host", cfg.ConnConfig.Host, "db", cfg.ConnConfig.Database)

	return pool, nil
}
