package hollow

import (
	"testing"
)

func TestDiffIsIdempotent(t *testing.T) {
	data := []byte("hello world")
	diff := Diff(data, data)
	if !diff.IsEmpty() {
		t.Fatal("expected empty diff for identical data")
	}
}

func TestDiffDetectsChanges(t *testing.T) {
	a := []byte("hello")
	b := []byte("world")
	diff := Diff(a, b)
	if diff.IsEmpty() {
		t.Fatal("expected non-empty diff for different data")
	}
}

func TestDiffDataWithAdditions(t *testing.T) {
	old := map[string]any{"key1": "value1"}
	new := map[string]any{"key1": "value1", "key2": "value2"}
	
	diff := DiffData(old, new)
	if diff.IsEmpty() {
		t.Fatal("expected non-empty diff")
	}
	
	if len(diff.added) != 1 || diff.added[0] != "key2" {
		t.Fatalf("expected added key2, got %v", diff.added)
	}
	
	if len(diff.removed) != 0 {
		t.Fatalf("expected no removed keys, got %v", diff.removed)
	}
	
	if len(diff.changed) != 0 {
		t.Fatalf("expected no changed keys, got %v", diff.changed)
	}
}

func TestDiffDataWithRemovals(t *testing.T) {
	old := map[string]any{"key1": "value1", "key2": "value2"}
	new := map[string]any{"key1": "value1"}
	
	diff := DiffData(old, new)
	if diff.IsEmpty() {
		t.Fatal("expected non-empty diff")
	}
	
	if len(diff.removed) != 1 || diff.removed[0] != "key2" {
		t.Fatalf("expected removed key2, got %v", diff.removed)
	}
	
	if len(diff.added) != 0 {
		t.Fatalf("expected no added keys, got %v", diff.added)
	}
	
	if len(diff.changed) != 0 {
		t.Fatalf("expected no changed keys, got %v", diff.changed)
	}
}

func TestDiffDataWithChanges(t *testing.T) {
	old := map[string]any{"key1": "value1"}
	new := map[string]any{"key1": "value2"}
	
	diff := DiffData(old, new)
	if diff.IsEmpty() {
		t.Fatal("expected non-empty diff")
	}
	
	if len(diff.changed) != 1 || diff.changed[0] != "key1" {
		t.Fatalf("expected changed key1, got %v", diff.changed)
	}
	
	if len(diff.added) != 0 {
		t.Fatalf("expected no added keys, got %v", diff.added)
	}
	
	if len(diff.removed) != 0 {
		t.Fatalf("expected no removed keys, got %v", diff.removed)
	}
}

func TestDiffDataIsEmpty(t *testing.T) {
	data := map[string]any{"key1": "value1", "key2": "value2"}
	diff := DiffData(data, data)
	if !diff.IsEmpty() {
		t.Fatal("expected empty diff for identical data")
	}
}

func TestDiffApply(t *testing.T) {
	old := map[string]any{"key1": "value1", "key2": "value2"}
	new := map[string]any{"key1": "modified", "key3": "value3"}
	
	diff := DiffData(old, new)
	target := make(map[string]any)
	for k, v := range old {
		target[k] = v
	}
	
	diff.Apply(target, new)
	
	// Check that target now matches new
	if len(target) != len(new) {
		t.Fatalf("expected target size %d, got %d", len(new), len(target))
	}
	
	for k, v := range new {
		if target[k] != v {
			t.Fatalf("expected target[%v] = %v, got %v", k, v, target[k])
		}
	}
	
	// Check that removed keys are gone
	if _, exists := target["key2"]; exists {
		t.Fatal("expected key2 to be removed")
	}
}

func FuzzDiffIsIdempotent(f *testing.F) {
	f.Add([]byte("hello"))
	f.Add([]byte("world"))
	f.Add([]byte(""))
	f.Add([]byte("a"))
	
	f.Fuzz(func(t *testing.T, b []byte) {
		diff := Diff(b, b)
		if !diff.IsEmpty() {
			t.Fatalf("expected empty diff for identical data: %v", string(b))
		}
	})
}

func FuzzDiffDetectsChanges(f *testing.F) {
	f.Add([]byte("hello"), []byte("world"))
	f.Add([]byte(""), []byte("test"))
	f.Add([]byte("test"), []byte(""))
	
	f.Fuzz(func(t *testing.T, a, b []byte) {
		diff := Diff(a, b)
		if string(a) == string(b) {
			if !diff.IsEmpty() {
				t.Fatalf("expected empty diff for identical data: %v vs %v", string(a), string(b))
			}
		} else {
			if diff.IsEmpty() {
				t.Fatalf("expected non-empty diff for different data: %v vs %v", string(a), string(b))
			}
		}
	})
}
