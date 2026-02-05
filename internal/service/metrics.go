package service

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"sixtyseven/internal/config"
	"sixtyseven/internal/domain"
	"sixtyseven/internal/repository/postgres"
)

// MetricsBroadcaster is an interface for broadcasting metrics updates
type MetricsBroadcaster interface {
	Broadcast(runID uuid.UUID, update domain.MetricUpdate)
}

// MetricsService handles metrics operations with buffering
type MetricsService struct {
	cfg         *config.Config
	repo        *postgres.MetricsRepository
	broadcaster MetricsBroadcaster

	// Buffering
	buffer    map[uuid.UUID][]domain.MetricPoint
	bufferMu  sync.Mutex
	batchSize int
	flushInterval time.Duration

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewMetricsService creates a new metrics service
func NewMetricsService(cfg *config.Config, repo *postgres.MetricsRepository, broadcaster MetricsBroadcaster) *MetricsService {
	s := &MetricsService{
		cfg:           cfg,
		repo:          repo,
		broadcaster:   broadcaster,
		buffer:        make(map[uuid.UUID][]domain.MetricPoint),
		batchSize:     cfg.MetricsBatchSize,
		flushInterval: cfg.MetricsFlushInterval,
		stopCh:        make(chan struct{}),
	}

	// Start background flusher
	s.wg.Add(1)
	go s.flushLoop()

	return s
}

// Stop stops the background flusher
func (s *MetricsService) Stop() {
	close(s.stopCh)
	s.wg.Wait()

	// Final flush
	s.flushAll(context.Background())
}

// LogMetrics logs a batch of metrics for a run
func (s *MetricsService) LogMetrics(ctx context.Context, runID uuid.UUID, metrics []domain.MetricPoint) error {
	now := time.Now()

	// Add to buffer
	s.bufferMu.Lock()
	for i := range metrics {
		if metrics[i].Timestamp.IsZero() {
			metrics[i].Timestamp = now
		}
	}
	s.buffer[runID] = append(s.buffer[runID], metrics...)
	bufferLen := len(s.buffer[runID])
	s.bufferMu.Unlock()

	// Broadcast to WebSocket clients
	if s.broadcaster != nil {
		for _, m := range metrics {
			s.broadcaster.Broadcast(runID, domain.MetricUpdate{
				RunID:     runID,
				Name:      m.Name,
				Step:      m.Step,
				Value:     m.Value,
				Timestamp: m.Timestamp,
			})
		}
	}

	// Flush if buffer is full
	if bufferLen >= s.batchSize {
		return s.flushRun(ctx, runID)
	}

	return nil
}

// GetSeries retrieves a metric time series
func (s *MetricsService) GetSeries(ctx context.Context, opts domain.MetricQueryOptions) ([]domain.Metric, error) {
	return s.repo.GetSeries(ctx, opts)
}

// GetLatest retrieves the latest value for each metric in a run
func (s *MetricsService) GetLatest(ctx context.Context, runID uuid.UUID) (map[string]float64, error) {
	return s.repo.GetLatest(ctx, runID)
}

// GetMetricNames retrieves all unique metric names for a run
func (s *MetricsService) GetMetricNames(ctx context.Context, runID uuid.UUID) ([]string, error) {
	return s.repo.GetMetricNames(ctx, runID)
}

// GetSummary retrieves summary statistics for all metrics in a run
func (s *MetricsService) GetSummary(ctx context.Context, runID uuid.UUID) (*domain.RunMetricsSummary, error) {
	return s.repo.GetSummary(ctx, runID)
}

// flushLoop periodically flushes buffered metrics
func (s *MetricsService) flushLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.flushAll(context.Background())
		case <-s.stopCh:
			return
		}
	}
}

// flushAll flushes all buffered metrics
func (s *MetricsService) flushAll(ctx context.Context) {
	s.bufferMu.Lock()
	runIDs := make([]uuid.UUID, 0, len(s.buffer))
	for runID := range s.buffer {
		runIDs = append(runIDs, runID)
	}
	s.bufferMu.Unlock()

	for _, runID := range runIDs {
		_ = s.flushRun(ctx, runID)
	}
}

// flushRun flushes buffered metrics for a specific run
func (s *MetricsService) flushRun(ctx context.Context, runID uuid.UUID) error {
	s.bufferMu.Lock()
	metrics := s.buffer[runID]
	delete(s.buffer, runID)
	s.bufferMu.Unlock()

	if len(metrics) == 0 {
		return nil
	}

	return s.repo.BatchInsert(ctx, runID, metrics)
}

// Flush forces a flush of all buffered metrics (useful for testing)
func (s *MetricsService) Flush(ctx context.Context) error {
	s.flushAll(ctx)
	return nil
}
