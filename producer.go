package hollow

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sync"
)

// ProducerOpt configures a Producer.
type ProducerOpt func(*Producer)

// WithBlobStager sets the blob stager for the producer.
func WithBlobStager(stager BlobStager) ProducerOpt {
	return func(p *Producer) {
		p.stager = stager
	}
}

// WithLogger sets the logger for the producer.
func WithLogger(logger *slog.Logger) ProducerOpt {
	return func(p *Producer) {
		p.logger = logger
	}
}

// WithMetricsCollector sets the metrics collector for the producer.
func WithMetricsCollector(collector MetricsCollector) ProducerOpt {
	return func(p *Producer) {
		p.metrics = collector
	}
}

// NewProducer creates a new Producer with the given options.
func NewProducer(opts ...ProducerOpt) *Producer {
	p := &Producer{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	
	for _, opt := range opts {
		opt(p)
	}
	
	return p
}

// RunCycle executes a write cycle and returns the new version.
func (p *Producer) RunCycle(fn func(WriteState) error) (uint64, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	ctx := context.Background()
	ws := &writeState{
		data: make(map[string]any),
	}
	
	if err := fn(ws); err != nil {
		return 0, err
	}
	
	// Serialize the data
	data, err := json.Marshal(ws.data)
	if err != nil {
		return 0, err
	}
	
	// Get next version
	version := p.version.Add(1)
	
	// Stage the data
	if err := p.stager.Stage(ctx, version, data); err != nil {
		return 0, err
	}
	
	// Commit the staged data
	if err := p.stager.Commit(ctx, version); err != nil {
		return 0, err
	}
	
	// Collect metrics
	if p.metrics != nil {
		p.metrics(Metrics{
			Version:     version,
			RecordCount: len(ws.data),
			ByteSize:    int64(len(data)),
		})
	}
	
	p.logger.Info("completed write cycle", "version", version, "records", len(ws.data))
	
	return version, nil
}

// writeState implements WriteState for producer operations.
type writeState struct {
	mu   sync.RWMutex
	data map[string]any
}

// Add adds a value to the write state.
func (ws *writeState) Add(v any) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	
	// Convert value to string for key
	key := fmt.Sprintf("%v", v)
	ws.data[key] = v
	return nil
}
