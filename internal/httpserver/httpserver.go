package httpserver

import (
	"context"
	"errors"
	"net/http"
	"time"

	"l0/internal/cache"
	"l0/internal/generate"
	"l0/internal/kafkaproducer"
	"l0/internal/storage"
	"log/slog"

	"github.com/gin-gonic/gin"
)

type HttpServer struct {
	http  *http.Server
	log   *slog.Logger
	repo  *storage.OrderRepository
	cache cache.OrderCache
	prod  *kafkaproducer.KafkaProducer
}

type Opts struct {
	Addr  string
	Log   *slog.Logger
	Repo  *storage.OrderRepository
	Cache cache.OrderCache
	Prod  *kafkaproducer.KafkaProducer
}

func NewHttpServer(opts Opts) *HttpServer {
	if opts.Log == nil {
		opts.Log = slog.Default()
	}

	r := gin.New()
	r.Use(gin.Recovery())

	// лог запросов
	r.Use(func(c *gin.Context) {
		start := time.Now()
		c.Next()
		opts.Log.Info("http",
			"method", c.Request.Method,
			"path", c.FullPath(),
			"status", c.Writer.Status(),
			"latency", time.Since(start).String(),
			"ip", c.ClientIP(),
		)
	})

	// healthz
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// GET /order/:id (оставляю как у тебя)
	r.GET("/order/:id", func(c *gin.Context) {
		id := c.Param("id")
		if o, ok := opts.Cache.Get(id); ok {
			c.JSON(http.StatusOK, o)
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		o, err := opts.Repo.GetOrder(ctx, id)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				c.JSON(http.StatusNotFound, gin.H{
					"error":       "order not found",
					"order_uid":   id,
					"description": "no order found with this id",
				})
				return
			}
			opts.Log.Error("get order failed", "err", err, "id", id)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		opts.Cache.Set(id, o)
		c.JSON(http.StatusOK, o)
	})

	// --- новые роуты генерации и отправки в Kafka ---

	// Валидный заказ → Kafka → вернуть JSON заказ
	r.GET("/generate_correct", func(c *gin.Context) {
		if opts.Prod == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "producer not configured"})
			return
		}
		order := generate.ValidOrder()

		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		if err := opts.Prod.ProduceOrder(ctx, order); err != nil {
			opts.Log.Error("kafka produce valid failed", "err", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "kafka produce failed"})
			return
		}

		// Можно сразу заполнить кэш, чтобы GET /order/:id отдал мгновенно
		opts.Cache.Set(order.OrderUID, order)

		c.JSON(http.StatusOK, order)
	})

	// Невалидный payload → Kafka → вернуть payload
	r.GET("/generate_incorrect", func(c *gin.Context) {
		if opts.Prod == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "producer not configured"})
			return
		}
		payload := generate.InvalidPayload()

		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		if err := opts.Prod.Produce(ctx, nil, payload); err != nil {
			opts.Log.Error("kafka produce invalid failed", "err", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "kafka produce failed"})
			return
		}

		// Возвращаем как есть (string), чтобы ты увидел «что ушло»
		c.Data(http.StatusOK, "application/json; charset=utf-8", payload)
	})

	srv := &http.Server{
		Addr:              opts.Addr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return &HttpServer{
		http:  srv,
		log:   opts.Log,
		repo:  opts.Repo,
		cache: opts.Cache,
		prod:  opts.Prod,
	}
}

func (s *HttpServer) Start() error {
	s.log.Info("start http server", "addr", s.http.Addr)

	// Запуск в горутине, чтоб вызывающий не ждал
	go func() {
		if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.log.Error("start http server failed", "err", err)
		}
	}()
	return nil
}

func (s *HttpServer) Stop(ctx context.Context) error {
	s.log.Info("stop http server", "addr", s.http.Addr)
	return s.http.Shutdown(ctx)
}
