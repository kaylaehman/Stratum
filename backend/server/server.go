package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Server wraps an http.Server with graceful shutdown.
type Server struct {
	http   *http.Server
	logger *slog.Logger
}

// New creates a Server listening on addr (e.g. ":8080") with the given handler.
func New(addr string, handler http.Handler, logger *slog.Logger) *Server {
	return &Server{
		http: &http.Server{
			Addr:              addr,
			Handler:           handler,
			ReadHeaderTimeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// Run starts the server and blocks until ctx is cancelled, then drains
// connections with a timeout. WebSocket handlers hold the request context, so
// cancelling ctx also tears down live streams.
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("http server listening", "addr", s.http.Addr)
		if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("server: listen: %w", err)
	case <-ctx.Done():
		s.logger.Info("shutdown signal received; draining")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := s.http.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server: shutdown: %w", err)
		}
		return nil
	}
}
