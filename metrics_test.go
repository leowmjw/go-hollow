package hollow

import (
	"errors"
	"testing"

	"github.com/leowmjw/go-hollow/legacy/internal/memblob"
)

func TestMetricsCollectorInvoked(t *testing.T) {
	collected := false
	var collectedMetrics Metrics
	collFn := func(m Metrics) {
		collected = true
		collectedMetrics = m
	}

	blob := memblob.New()
	p := NewProducer(
		WithBlobStager(blob),
		WithMetricsCollector(collFn),
	)
	version, err := p.RunCycle(func(ws WriteState) error { return ws.Add(7) })
	if err != nil {
		t.Fatalf("cycle failed: %v", err)
	}

	if !collected {
		t.Fatal("metrics not collected")
	}

	if collectedMetrics.Version != version {
		t.Fatalf("expected version %d, got %d", version, collectedMetrics.Version)
	}

	if collectedMetrics.RecordCount != 1 {
		t.Fatalf("expected record count 1, got %d", collectedMetrics.RecordCount)
	}

	if collectedMetrics.ByteSize <= 0 {
		t.Fatalf("expected positive byte size, got %d", collectedMetrics.ByteSize)
	}
}

func TestMetricsCollectorWithMultipleItems(t *testing.T) {
	var collectedMetrics Metrics
	collFn := func(m Metrics) { collectedMetrics = m }

	blob := memblob.New()
	p := NewProducer(
		WithBlobStager(blob),
		WithMetricsCollector(collFn),
	)

	_, err := p.RunCycle(func(ws WriteState) error {
		if err := ws.Add("item1"); err != nil {
			return err
		}
		if err := ws.Add("item2"); err != nil {
			return err
		}
		return ws.Add("item3")
	})
	if err != nil {
		t.Fatalf("cycle failed: %v", err)
	}

	if collectedMetrics.RecordCount != 3 {
		t.Fatalf("expected record count 3, got %d", collectedMetrics.RecordCount)
	}
}

func TestMetricsCollectorNotInvokedOnError(t *testing.T) {
	collected := false
	collFn := func(m Metrics) { collected = true }

	blob := memblob.New()
	p := NewProducer(
		WithBlobStager(blob),
		WithMetricsCollector(collFn),
	)

	// This should fail and not collect metrics
	_, err := p.RunCycle(func(ws WriteState) error {
		return errors.New("test error")
	})
	if err == nil {
		t.Fatal("expected error from cycle")
	}

	if collected {
		t.Fatal("metrics should not be collected on error")
	}
}
