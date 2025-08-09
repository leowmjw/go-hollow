package internal

import (
	"testing"
)

type testUser struct {
	ID   int64
	Name string
}

func TestPKStableShardAssignmentAcrossCycles(t *testing.T) {
	wse := NewWriteStateEngine()

	// Configure PK for testUser
	typeName := GetTypeName(&testUser{})
	wse.SetPrimaryKeys(map[string]string{typeName: "ID"})
	wse.SetNumShards(typeName, 8)

	// Cycle 1
	wse.PrepareForCycle()
	if err := wse.AddWithPrimaryKey(&testUser{ID: 1001, Name: "alice"}); err != nil { t.Fatal(err) }
	if err := wse.AddWithPrimaryKey(&testUser{ID: 1002, Name: "bob"}); err != nil { t.Fatal(err) }
	if err := wse.AddWithPrimaryKey(&testUser{ID: 1003, Name: "charlie"}); err != nil { t.Fatal(err) }

	records1, ok := wse.currentState.data[typeName]
	if !ok || len(records1) != 3 {
		t.Fatalf("expected 3 records in cycle 1, got %d", len(records1))
	}
	for i, rec := range records1 {
		ri, ok := rec.(*RecordInfo)
		if !ok {
			t.Fatalf("record %d not RecordInfo", i)
		}
		key, err := wse.extractPrimaryKeyValue(ri.Value, "ID", typeName)
		if err != nil { t.Fatal(err) }
		expected := int(Hash64String(key) % uint64(8))
		if ri.Shard != expected {
			t.Fatalf("cycle1: expected shard %d, got %d for key %s", expected, ri.Shard, key)
		}
	}

	// Cycle 2 with same PKs (updates) -> should map to same shards for same key
	wse.PrepareForCycle()
	if err := wse.AddWithPrimaryKey(&testUser{ID: 1001, Name: "aliceV2"}); err != nil { t.Fatal(err) }
	if err := wse.AddWithPrimaryKey(&testUser{ID: 1002, Name: "bobV2"}); err != nil { t.Fatal(err) }
	if err := wse.AddWithPrimaryKey(&testUser{ID: 1003, Name: "charlieV2"}); err != nil { t.Fatal(err) }

	records2, ok := wse.currentState.data[typeName]
	if !ok || len(records2) != 3 {
		t.Fatalf("expected 3 records in cycle 2, got %d", len(records2))
	}
	for i, rec := range records2 {
		ri, ok := rec.(*RecordInfo)
		if !ok {
			t.Fatalf("record %d not RecordInfo", i)
		}
		key, err := wse.extractPrimaryKeyValue(ri.Value, "ID", typeName)
		if err != nil { t.Fatal(err) }
		expected := int(Hash64String(key) % uint64(8))
		if ri.Shard != expected {
			t.Fatalf("cycle2: expected shard %d, got %d for key %s", expected, ri.Shard, key)
		}
	}
}

func TestPKShardRecomputeAfterResharding(t *testing.T) {
	wse := NewWriteStateEngine()
	typeName := GetTypeName(&testUser{})
	wse.SetPrimaryKeys(map[string]string{typeName: "ID"})

	// Start with 4 shards and add records
	wse.SetNumShards(typeName, 4)
	wse.PrepareForCycle()
	if err := wse.AddWithPrimaryKey(&testUser{ID: 2001, Name: "a"}); err != nil { t.Fatal(err) }
	if err := wse.AddWithPrimaryKey(&testUser{ID: 2002, Name: "b"}); err != nil { t.Fatal(err) }
	if err := wse.AddWithPrimaryKey(&testUser{ID: 2003, Name: "c"}); err != nil { t.Fatal(err) }

	records, ok := wse.currentState.data[typeName]
	if !ok || len(records) != 3 { t.Fatalf("expected 3 records, got %d", len(records)) }
	for i, rec := range records {
		ri, ok := rec.(*RecordInfo)
		if !ok { t.Fatalf("record %d not RecordInfo", i) }
		key, err := wse.extractPrimaryKeyValue(ri.Value, "ID", typeName)
		if err != nil { t.Fatal(err) }
		expected := int(Hash64String(key) % uint64(4))
		if ri.Shard != expected {
			t.Fatalf("before reshard: expected shard %d, got %d for key %s", expected, ri.Shard, key)
		}
	}

	// Increase shard count and recompute
	wse.SetNumShards(typeName, 16)
	wse.RecomputeShardAssignments()

	records, _ = wse.currentState.data[typeName]
	for i, rec := range records {
		ri, ok := rec.(*RecordInfo)
		if !ok { t.Fatalf("record %d not RecordInfo", i) }
		key, err := wse.extractPrimaryKeyValue(ri.Value, "ID", typeName)
		if err != nil { t.Fatal(err) }
		expected := int(Hash64String(key) % uint64(16))
		if ri.Shard != expected {
			t.Fatalf("after reshard: expected shard %d, got %d for key %s", expected, ri.Shard, key)
		}
	}
}
