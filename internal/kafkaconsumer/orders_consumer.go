package kafkaconsumer

import (
	"context"
	"encoding/json"
	"errors"
	"l0/internal/cache"
	"l0/internal/model"
	"l0/internal/storage"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"
)

type KafkaConsumer struct {
	reader *kafka.Reader
	repo   *storage.OrderRepository
	cache  cache.OrderCache
	log    *slog.Logger
}

type KafkaConsumerConfig struct {
	Brokers []string
	Topic   string
	GroupID string
}

func NewKafkaConsumer(conf KafkaConsumerConfig, repo *storage.OrderRepository, c cache.OrderCache, log *slog.Logger) *KafkaConsumer {
	if log == nil {
		log = slog.Default()
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        conf.Brokers,
		Topic:          conf.Topic,
		GroupID:        conf.GroupID,
		MinBytes:       1,
		MaxBytes:       10e6,
		MaxWait:        500 * time.Millisecond,
		CommitInterval: 1 * time.Second,
	})
	return &KafkaConsumer{reader: reader, repo: repo, cache: c, log: log}
}

func (kc *KafkaConsumer) Close() error {
	return kc.reader.Close()
}

func (kc *KafkaConsumer) Run(ctx context.Context) error {
	kc.log.Info("kafka consumer starting",
		"topic", kc.reader.Config().Topic,
		"group", kc.reader.Config().GroupID,
		"brokers", kc.reader.Config().Brokers,
	)

	for {
		m, err := kc.reader.ReadMessage(ctx)
		if err != nil {
			// контекст закрыли — выходим мягко
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				kc.log.Info("kafka consumer context closed, stopping")
				return nil
			}
			kc.log.Error("kafka ReadMessage error", "err", err)
			// короткий backoff, чтобы не крутить CPU при проблемах с сетью
			select {
			case <-time.After(500 * time.Millisecond):
				continue
			case <-ctx.Done():
				return nil
			}
		}

		var ord model.Order
		if err := json.Unmarshal(m.Value, &ord); err != nil {
			kc.log.Error("unmarshal order failed", "err", err, "key", string(m.Key))
			continue
		}

		saveCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		err = kc.repo.SaveOrder(saveCtx, ord)
		cancel()
		if err != nil {
			kc.log.Error("save order failed", "err", err, "order_uid", ord.OrderUID)
			continue
		}

		// backfill cache (на чтение отдастся мгновенно)
		kc.cache.Set(ord.OrderUID, ord)

		kc.log.Info("order stored from kafka",
			"order_uid", ord.OrderUID,
			"topic", kc.reader.Config().Topic,
			"partition", m.Partition,
			"offset", m.Offset,
		)
	}
}
