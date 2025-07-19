package hollow

import (
	"io"
	"log/slog"
	"testing"

	"github.com/leowmjw/go-hollow/internal/memblob"
)

func TestProducerRunsSnapshotCycle(t *testing.T) {
	blob := memblob.New()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	p := NewProducer(
		WithBlobStager(blob),
		WithLogger(logger),
	)

	v, err := p.RunCycle(func(ws WriteState) error {
		return ws.Add(1)
	})
	if err != nil {
		t.Fatalf("cycle failed: %v", err)
	}
	if v == 0 {
		t.Fatal("expected non-zero version")
	}
}

func TestProducerMultipleCycles(t *testing.T) {
	blob := memblob.New()
	p := NewProducer(WithBlobStager(blob))

	// First cycle
	v1, err := p.RunCycle(func(ws WriteState) error {
		return ws.Add("item1")
	})
	if err != nil {
		t.Fatalf("first cycle failed: %v", err)
	}

	// Second cycle
	v2, err := p.RunCycle(func(ws WriteState) error {
		return ws.Add("item2")
	})
	if err != nil {
		t.Fatalf("second cycle failed: %v", err)
	}

	if v2 <= v1 {
		t.Fatalf("expected v2 (%d) > v1 (%d)", v2, v1)
	}
}

func TestProducerWithMultipleItems(t *testing.T) {
	blob := memblob.New()
	p := NewProducer(WithBlobStager(blob))

	v, err := p.RunCycle(func(ws WriteState) error {
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
	if v == 0 {
		t.Fatal("expected non-zero version")
	}
}

func TestProducerCycleError(t *testing.T) {
	blob := memblob.New()
	p := NewProducer(WithBlobStager(blob))

	_, err := p.RunCycle(func(ws WriteState) error {
		return io.EOF // simulate error
	})
	if err == nil {
		t.Fatal("expected error from failed cycle")
	}
}
