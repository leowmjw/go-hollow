package hollow

import (
	"bytes"
	"reflect"
)

// Diff computes the difference between two byte slices.
func Diff(a, b []byte) *DataDiff {
	if bytes.Equal(a, b) {
		return &DataDiff{}
	}
	
	// For now, treat any difference as a complete change
	// In a real implementation, this would be more sophisticated
	return &DataDiff{
	changed: []string{string(b)},
	}
}

// DiffData computes the difference between two data maps.
func DiffData(old, new map[string]any) *DataDiff {
	diff := &DataDiff{}
	
	// Find added and changed items
	for k, v := range new {
		if oldV, exists := old[k]; !exists {
			diff.added = append(diff.added, k)
		} else if !reflect.DeepEqual(oldV, v) {
			diff.changed = append(diff.changed, k)
		}
	}
	
	// Find removed items
	for k := range old {
		if _, exists := new[k]; !exists {
			diff.removed = append(diff.removed, k)
		}
	}
	
	return diff
}

// Apply applies a diff to a data map (simplified implementation).
func (d *DataDiff) Apply(target map[string]any, source map[string]any) {
	// Add new items
	for _, key := range d.added {
		if val, exists := source[key]; exists {
			target[key] = val
		}
	}
	
	// Update changed items
	for _, key := range d.changed {
		if val, exists := source[key]; exists {
			target[key] = val
		}
	}
	
	// Remove deleted items
	for _, key := range d.removed {
		delete(target, key)
	}
}
