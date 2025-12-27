package producer

import (
	"context"
	"fmt"
	"testing"

	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/internal"
)

// integOrder is a second type to validate multi-type PK sharding behavior.
type integOrder struct {
	OrderID string
	UserID  int64
}

// TestProducerMultiTypeRepeatedResharding verifies that shard assignments are recomputed
// correctly for multiple types across repeated resharding events.
func TestProducerMultiTypeRepeatedResharding(t *testing.T) {
	store := blob.NewInMemoryBlobStore()
	ctx := context.Background()

	userType := internal.GetTypeName(&integUser{})
	orderType := internal.GetTypeName(&integOrder{})

	p := NewProducer(
		WithBlobStore(store),
		WithTypeResharding(true),
		WithTargetMaxTypeShardSize(3), // small to force resharding
		WithPrimaryKey(userType, "ID"),
		WithPrimaryKey(orderType, "OrderID"),
	)

	// Cycle 1: users=10 => shards=ceil(10/3)=4; orders=8 => shards=ceil(8/3)=3
	v1, err := p.RunCycle(ctx, func(ws *internal.WriteState) {
		for i := 0; i < 10; i++ {
			ws.Add(&integUser{ID: int64(1000 + i), Name: fmt.Sprintf("u%d", i)})
		}
		for i := 0; i < 8; i++ {
			ws.Add(&integOrder{OrderID: fmt.Sprintf("O-%d", i), UserID: int64(1000 + (i % 10))})
		}
	})
	if err != nil {
		t.Fatalf("cycle1 failed: %v", err)
	}
	if v1 == 0 {
		t.Fatalf("expected non-zero version for cycle1")
	}

	engine := p.GetWriteEngine()

	// Validate cycle 1 shard assignments per type
	{
		expectedShardsUsers := 4
		records := engine.GetWriteState().GetData()[userType]
		if len(records) != 10 {
			t.Fatalf("cycle1 users count: expected 10 got %d", len(records))
		}
		for i, rec := range records {
			ri, ok := rec.(*internal.RecordInfo)
			if !ok {
				t.Fatalf("users record %d not *internal.RecordInfo", i)
			}
			user := ri.Value.(*integUser)
			key := fmt.Sprintf("%v", user.ID)
			expected := int(internal.Hash64String(key) % uint64(expectedShardsUsers))
			if ri.Shard != expected {
				t.Fatalf("users cycle1 shard mismatch: key=%s shard=%d expected=%d", key, ri.Shard, expected)
			}
			if ri.Shard < 0 || ri.Shard >= expectedShardsUsers {
				t.Fatalf("users cycle1 shard out of range: %d not in [0,%d)", ri.Shard, expectedShardsUsers)
			}
		}
	}

	{
		expectedShardsOrders := 3
		records := engine.GetWriteState().GetData()[orderType]
		if len(records) != 8 {
			t.Fatalf("cycle1 orders count: expected 8 got %d", len(records))
		}
		for i, rec := range records {
			ri, ok := rec.(*internal.RecordInfo)
			if !ok {
				t.Fatalf("orders record %d not *internal.RecordInfo", i)
			}
			ord := ri.Value.(*integOrder)
			key := fmt.Sprintf("%v", ord.OrderID)
			expected := int(internal.Hash64String(key) % uint64(expectedShardsOrders))
			if ri.Shard != expected {
				t.Fatalf("orders cycle1 shard mismatch: key=%s shard=%d expected=%d", key, ri.Shard, expected)
			}
			if ri.Shard < 0 || ri.Shard >= expectedShardsOrders {
				t.Fatalf("orders cycle1 shard out of range: %d not in [0,%d)", ri.Shard, expectedShardsOrders)
			}
		}
	}

	// Cycle 2: add more, users total=16 => shards=ceil(16/3)=6; orders total=12 => shards=ceil(12/3)=4
	v2, err := p.RunCycle(ctx, func(ws *internal.WriteState) {
		// re-add existing + new to represent full state
		for i := 0; i < 16; i++ {
			ws.Add(&integUser{ID: int64(1000 + i), Name: fmt.Sprintf("u%d_v2", i)})
		}
		for i := 0; i < 12; i++ {
			ws.Add(&integOrder{OrderID: fmt.Sprintf("O-%d", i), UserID: int64(1000 + (i % 16))})
		}
	})
	if err != nil {
		t.Fatalf("cycle2 failed: %v", err)
	}
	if v2 == 0 {
		t.Fatalf("expected non-zero version for cycle2")
	}

	// Validate cycle 2 shard assignments per type
	{
		expectedShardsUsers := 6
		records := engine.GetWriteState().GetData()[userType]
		if len(records) != 16 {
			t.Fatalf("cycle2 users count: expected 16 got %d", len(records))
		}
		for i, rec := range records {
			ri, ok := rec.(*internal.RecordInfo)
			if !ok {
				t.Fatalf("users record %d not *internal.RecordInfo", i)
			}
			user := ri.Value.(*integUser)
			key := fmt.Sprintf("%v", user.ID)
			expected := int(internal.Hash64String(key) % uint64(expectedShardsUsers))
			if ri.Shard != expected {
				t.Fatalf("users cycle2 shard mismatch: key=%s shard=%d expected=%d", key, ri.Shard, expected)
			}
			if ri.Shard < 0 || ri.Shard >= expectedShardsUsers {
				t.Fatalf("users cycle2 shard out of range: %d not in [0,%d)", ri.Shard, expectedShardsUsers)
			}
		}
	}

	{
		expectedShardsOrders := 4
		records := engine.GetWriteState().GetData()[orderType]
		if len(records) != 12 {
			t.Fatalf("cycle2 orders count: expected 12 got %d", len(records))
		}
		for i, rec := range records {
			ri, ok := rec.(*internal.RecordInfo)
			if !ok {
				t.Fatalf("orders record %d not *internal.RecordInfo", i)
			}
			ord := ri.Value.(*integOrder)
			key := fmt.Sprintf("%v", ord.OrderID)
			expected := int(internal.Hash64String(key) % uint64(expectedShardsOrders))
			if ri.Shard != expected {
				t.Fatalf("orders cycle2 shard mismatch: key=%s shard=%d expected=%d", key, ri.Shard, expected)
			}
			if ri.Shard < 0 || ri.Shard >= expectedShardsOrders {
				t.Fatalf("orders cycle2 shard out of range: %d not in [0,%d)", ri.Shard, expectedShardsOrders)
			}
		}
	}
}
