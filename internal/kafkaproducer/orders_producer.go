package kafkaproducer

import (
	"context"
	"encoding/json"
	"time"

	"l0/internal/model"
	"log/slog"

	"github.com/segmentio/kafka-go"
)

type KafkaProducer struct {
	writer *kafka.Writer
	log    *slog.Logger
}

type Config struct {
	Brokers []string
	Topic   string
}

func NewKafkaProducer(conf Config, log *slog.Logger) *KafkaProducer {
	if log == nil {
		log = slog.Default()
	}
	w := kafka.NewWriter(kafka.WriterConfig{
		Brokers:      conf.Brokers,
		Topic:        conf.Topic,
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
		Async:        false,
	})
	return &KafkaProducer{writer: w, log: log}
}

func (p *KafkaProducer) Close() error {
	return p.writer.Close()
}

// Produce сырые данные
func (p *KafkaProducer) Produce(ctx context.Context, key []byte, value []byte) error {
	msg := kafka.Message{Key: key, Value: value, Time: time.Now()}
	return p.writer.WriteMessages(ctx, msg)
}

// ProduceOrder — удобный helper для корректного заказа
func (p *KafkaProducer) ProduceOrder(ctx context.Context, o model.Order) error {
	b, err := json.Marshal(o)
	if err != nil {
		return err
	}
	key := []byte(o.OrderUID)
	return p.Produce(ctx, key, b)
}
