// internal/app/run.go
package app

import (
	"context"
	"fmt"
	"l0/internal/cache"
	"l0/internal/config"
	"l0/internal/httpserver"
	"l0/internal/kafkaconsumer"
	"l0/internal/storage"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
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
	orderCache := cache.NewOrderTTLCache(5 * time.Minute)

	srv := httpserver.NewHttpServer(httpserver.Opts{
		Addr:  cfg.HttpServer.Address,
		Log:   log,
		Repo:  repo,
		Cache: orderCache,
	})
	if err = srv.Start(); err != nil {
		log.Error("server start failed", "err", err)
		os.Exit(1)
	}

	brokers := cfg.Kafka.Brokers
	topic := cfg.Kafka.Topic
	group := cfg.Kafka.GroupId
	if brokers == "" || topic == "" || group == "" {
		log.Error("kafka env not set",
			"KAFKA_BROKERS", brokers,
			"KAFKA_TOPIC_ORDERS", topic,
			"KAFKA_GROUP_ID", group,
		)
		os.Exit(1)
	}

	kc := kafkaconsumer.NewKafkaConsumer(
		kafkaconsumer.KafkaConsumerConfig{
			Brokers: strings.Split(brokers, ","),
			Topic:   topic,
			GroupID: group,
		},
		repo,
		orderCache,
		log,
	)

	// Запускаем consumer
	go func() {
		if err := kc.Run(rootCtx); err != nil {
			log.Error("kafka consumer stopped with error", "err", err)
			// решение: можно инициировать остановку всего сервиса
			// stop()
		}
	}()

	log.Info("ready; waiting for shutdown signal", "addr", cfg.HttpServer.Address)
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
