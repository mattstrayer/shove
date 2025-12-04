package server

import (
	"context"
	"fmt"
	"net/http"

	"log/slog"

	"github.com/mattstrayer/shove/internal/queue"
	"github.com/mattstrayer/shove/internal/services"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server ...
type Server struct {
	server        *http.Server
	shuttingDown  bool
	workerOnly    bool
	queueFactory  queue.QueueFactory
	feedbackStore queue.FeedbackStore
	workers       map[string]*worker
}

// NewServer ...
func NewServer(addr string, qf queue.QueueFactory, fs queue.FeedbackStore, workerOnly bool) (s *Server) {
	s = &Server{
		queueFactory:  qf,
		feedbackStore: fs,
		workerOnly:    workerOnly,
		workers:       make(map[string]*worker),
	}

	if !workerOnly {
		mux := http.NewServeMux()
		s.server = &http.Server{
			Addr:    addr,
			Handler: mux,
		}
		mux.HandleFunc("/api/push/", s.handlePush)
		mux.HandleFunc("/api/feedback", s.handleFeedback)
		mux.HandleFunc("/api/feedback/peek", s.handleFeedbackPeek)
		mux.Handle("/metrics", promhttp.Handler())
		mux.HandleFunc("/health", s.handleHealth)
	}
	return s
}

// Serve starts the HTTP server. Returns immediately if in worker-only mode.
func (s *Server) Serve() (err error) {
	if s.workerOnly {
		return nil
	}
	slog.Info("Shove server started")
	err = s.server.ListenAndServe()
	if s.shuttingDown {
		err = nil
	}
	return
}

// Shutdown ...
func (s *Server) Shutdown(ctx context.Context) (err error) {
	s.shuttingDown = true

	if s.server != nil {
		if err = s.server.Shutdown(ctx); err != nil {
			slog.Error("Shutting down Shove server", "error", err)
			return
		}
		slog.Info("Shove server stopped")
	}

	for _, w := range s.workers {
		err = w.shutdown()
		if err != nil {
			return
		}
	}
	if s.feedbackStore != nil {
		if err = s.feedbackStore.Close(); err != nil {
			slog.Error("Failed to close feedback store", "error", err)
		}
	}
	return
}

// AddService ...
func (s *Server) AddService(pp services.PushService, workers int, squash services.SquashConfig) (err error) {
	serviceID := pp.ID()
	slog.Info("Initializing service", "service", serviceID, "workers", workers, "queue", fmt.Sprintf("shove:%s", serviceID))
	q, err := s.queueFactory.NewQueue(serviceID)
	if err != nil {
		return
	}
	w, err := newWorker(pp, q)
	if err != nil {
		return
	}
	go w.serve(workers, squash, s)
	s.workers[serviceID] = w
	slog.Info("Service started", "service", serviceID, "workers", workers)
	return
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
