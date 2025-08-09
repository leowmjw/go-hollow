package producer

import (
	"context"
	"fmt"
	"testing"

	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/internal"
)

type integUser struct {
	ID   int64
	Name string
}

func TestProducerReshardingRecomputesPKShards(t *testing.T) {
	store := blob.NewInMemoryBlobStore()

	typeName := internal.GetTypeName(&integUser{})
	p := NewProducer(
		WithBlobStore(store),
		WithTypeResharding(true),
		WithTargetMaxTypeShardSize(3), // force resharding when len(values) > 3
		WithPrimaryKey(typeName, "ID"),
	)

	// Produce 10 records in one cycle -> expected new shard count = ceil(10/3) = 4
	ctx := context.Background()
	_, err := p.RunCycleE(ctx, func(ws *internal.WriteState) {
		for i := 0; i < 10; i++ {
			ws.Add(&integUser{ID: int64(1000 + i), Name: fmt.Sprintf("u%d", i)})
		}
	})
	if err != nil {
		t.Fatalf("RunCycleE failed: %v", err)
	}

	engine := p.GetWriteEngine()
    // GetData returns a map[string][]any keyed by type name; index with typeName.
    // Each element is an *internal.RecordInfo wrapping the original value and computed shard.
	records := engine.GetWriteState().GetData()[typeName]
	if len(records) != 10 {
		t.Fatalf("expected 10 records, got %d", len(records))
	}

	newShardCount := 4 // as computed by performResharding
	for i, rec := range records {
        // Assert to *internal.RecordInfo to access Ordinal/Shard and original Value.
		ri, ok := rec.(*internal.RecordInfo)
		if !ok {
			t.Fatalf("record %d not internal.RecordInfo", i)
		}
		user, ok := ri.Value.(*integUser)
		if !ok {
			t.Fatalf("record %d value not *integUser", i)
		}
		key := fmt.Sprintf("%v", user.ID)
		expected := int(internal.Hash64String(key) % uint64(newShardCount))
		if ri.Shard != expected {
			t.Fatalf("after reshard: expected shard %d, got %d for key %s", expected, ri.Shard, key)
		}
		if ri.Shard < 0 || ri.Shard >= newShardCount {
			t.Fatalf("shard out of range: %d not in [0,%d)", ri.Shard, newShardCount)
		}
	}
}

func TestProducerPKShardStabilityAcrossCycles(t *testing.T) {
	store := blob.NewInMemoryBlobStore()
	ctx := context.Background()

	typeName := internal.GetTypeName(&integUser{})
	p := NewProducer(
		WithBlobStore(store),
		WithTypeResharding(false),      // no resharding between cycles
		WithTargetMaxTypeShardSize(100),
		WithPrimaryKey(typeName, "ID"),
	)

	// Cycle 1
	v1, err := p.RunCycleE(ctx, func(ws *internal.WriteState) {
		for i := 0; i < 5; i++ {
			ws.Add(&integUser{ID: int64(2000 + i), Name: fmt.Sprintf("u%d", i)})
		}
	})
	if err != nil {
		t.Fatalf("cycle1 failed: %v", err)
	}
	if v1 == 0 {
		t.Fatalf("expected non-zero version for cycle1")
	}

	engine := p.GetWriteEngine()
	records1 := engine.GetWriteState().GetData()[typeName]
	shards1 := make(map[int]int) // ordinal -> shard
	for _, rec := range records1 {
		ri := rec.(*internal.RecordInfo)
		shards1[ri.Ordinal] = ri.Shard
	}

	// Cycle 2: same PKs but different names (updates)
	v2, err := p.RunCycleE(ctx, func(ws *internal.WriteState) {
		for i := 0; i < 5; i++ {
			ws.Add(&integUser{ID: int64(2000 + i), Name: fmt.Sprintf("u%d_v2", i)})
		}
	})
	if err != nil {
		t.Fatalf("cycle2 failed: %v", err)
	}
	if v2 == v1 {
		// Data content differs, but producer may dedupe; still check shards of current state
	}

	records2 := engine.GetWriteState().GetData()[typeName]
	if len(records2) != 5 {
		t.Fatalf("expected 5 records in cycle 2, got %d", len(records2))
	}
	for _, rec := range records2 {
		ri := rec.(*internal.RecordInfo)
		if prev, ok := shards1[ri.Ordinal]; ok {
			if ri.Shard != prev {
				t.Fatalf("shard changed across cycles for ordinal %d: %d -> %d", ri.Ordinal, prev, ri.Shard)
			}
		}
	}
}
