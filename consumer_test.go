package hollow

import (
	"testing"

	"github.com/leowmjw/go-hollow/internal/memblob"
)

func TestConsumerRefreshToVersion(t *testing.T) {
	blob := memblob.New()
	prod := NewProducer(WithBlobStager(blob))
	want, err := prod.RunCycle(func(ws WriteState) error { return ws.Add("x") })
	if err != nil {
		t.Fatalf("producer cycle failed: %v", err)
	}

	cons := NewConsumer(
		WithBlobRetriever(blob),
		WithAnnouncementWatcher(func() (uint64, bool, error) { return want, true, nil }),
	)
	if err := cons.Refresh(); err != nil {
		t.Fatal(err)
	}
	if got := cons.CurrentVersion(); got != want {
		t.Fatalf("got version %d, want %d", got, want)
	}
}

func TestConsumerRefreshWithNoNewVersion(t *testing.T) {
	blob := memblob.New()
	cons := NewConsumer(
		WithBlobRetriever(blob),
		WithAnnouncementWatcher(func() (uint64, bool, error) { return 0, false, nil }),
	)

	if err := cons.Refresh(); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	if got := cons.CurrentVersion(); got != 0 {
		t.Fatalf("expected version 0, got %d", got)
	}
}

func TestConsumerReadState(t *testing.T) {
	blob := memblob.New()
	prod := NewProducer(WithBlobStager(blob))

	// Produce some data
	version, err := prod.RunCycle(func(ws WriteState) error {
		if err := ws.Add("key1"); err != nil {
			return err
		}
		return ws.Add("key2")
	})
	if err != nil {
		t.Fatalf("producer cycle failed: %v", err)
	}

	// Consume the data
	cons := NewConsumer(
		WithBlobRetriever(blob),
		WithAnnouncementWatcher(func() (uint64, bool, error) { return version, true, nil }),
	)

	if err := cons.Refresh(); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	// Test read state
	rs := cons.ReadState()
	if size := rs.Size(); size != 2 {
		t.Fatalf("expected size 2, got %d", size)
	}

	// Test individual reads
	if val, ok := rs.Get("key1"); !ok || val != "key1" {
		t.Fatalf("expected key1 to exist with value 'key1', got %v, %v", val, ok)
	}

	if val, ok := rs.Get("key2"); !ok || val != "key2" {
		t.Fatalf("expected key2 to exist with value 'key2', got %v, %v", val, ok)
	}

	if _, ok := rs.Get("nonexistent"); ok {
		t.Fatal("expected nonexistent key to not exist")
	}
}

func TestConsumerMultipleRefresh(t *testing.T) {
	blob := memblob.New()
	prod := NewProducer(WithBlobStager(blob))

	// First version
	v1, err := prod.RunCycle(func(ws WriteState) error {
		return ws.Add("v1")
	})
	if err != nil {
		t.Fatalf("first cycle failed: %v", err)
	}

	// Second version
	v2, err := prod.RunCycle(func(ws WriteState) error {
		return ws.Add("v2")
	})
	if err != nil {
		t.Fatalf("second cycle failed: %v", err)
	}

	currentVersion := v1
	cons := NewConsumer(
		WithBlobRetriever(blob),
		WithAnnouncementWatcher(func() (uint64, bool, error) { return currentVersion, true, nil }),
	)

	// Refresh to v1
	if err := cons.Refresh(); err != nil {
		t.Fatalf("refresh to v1 failed: %v", err)
	}
	if got := cons.CurrentVersion(); got != v1 {
		t.Fatalf("expected version %d, got %d", v1, got)
	}

	// Refresh to v2
	currentVersion = v2
	if err := cons.Refresh(); err != nil {
		t.Fatalf("refresh to v2 failed: %v", err)
	}
	if got := cons.CurrentVersion(); got != v2 {
		t.Fatalf("expected version %d, got %d", v2, got)
	}
}
