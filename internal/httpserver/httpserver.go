package httpserver

import (
	"context"
	"errors"
	"l0/internal/cache"
	"l0/internal/storage"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type HttpServer struct {
	http  *http.Server
	log   *slog.Logger
	repo  *storage.OrderRepository
	cache cache.OrderCache
}

type Opts struct {
	Addr  string
	Log   *slog.Logger
	Repo  *storage.OrderRepository
	Cache cache.OrderCache
}

func NewHttpServer(opts Opts) *HttpServer {
	if opts.Log == nil {
		opts.Log = slog.Default()
	}

	r := gin.New()
	r.Use(gin.Recovery())

	// Лог запросов
	r.Use(func(c *gin.Context) {
		start := time.Now()
		c.Next()
		opts.Log.Info(
			"http",
			"method", c.Request.Method,
			"path", c.FullPath(),
			"status", c.Writer.Status(),
			"latency", time.Since(start).String(),
			"ip", c.ClientIP())
	})

	// healthz
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// order/id
	r.GET("order/:id", func(c *gin.Context) {
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
