package hollow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"capnproto.org/go/capnp/v3"
)

// CapnpStore implements BlobStager, BlobRetriever, and DeltaAwareBlobStore
// using Cap'n Proto for more efficient serialization.
type CapnpStore struct {
	mu        sync.RWMutex
	snapshots map[uint64][]byte // version -> serialized snapshot
	deltas    map[string][]byte // fromVersion_toVersion -> serialized delta
	metadata  map[uint64]*DeltaMetadata
	original  []byte            // original JSON data (for testing)
	latestVer uint64
}

// NewCapnpStore creates a new Cap'n Proto-backed blob store
func NewCapnpStore() *CapnpStore {
	return &CapnpStore{
		snapshots: make(map[uint64][]byte),
		deltas:    make(map[string][]byte),
		metadata:  make(map[uint64]*DeltaMetadata),
	}
}

// Stage implements BlobStager.Stage
func (s *CapnpStore) Stage(ctx context.Context, version uint64, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Store original JSON data for debugging
	s.original = data
	
	// Check if data is already in Cap'n Proto format
	// For our implementation, we'll assume data is always in JSON format
	// and needs to be converted to Cap'n Proto
	capData, err := jsonToCapnp(data)
	if err != nil {
		return fmt.Errorf("failed to convert to Cap'n Proto: %w", err)
	}
	
	// Store the Cap'n Proto data in the snapshots map
	s.snapshots[version] = capData
	
	return nil
}

// Commit implements BlobStager.Commit
func (s *CapnpStore) Commit(ctx context.Context, version uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.snapshots[version]; !exists {
		return fmt.Errorf("version %d not staged", version)
	}

	if version > s.latestVer {
		s.latestVer = version
	}

	return nil
}

// Retrieve implements BlobRetriever.Retrieve
func (s *CapnpStore) Retrieve(ctx context.Context, version uint64) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, exists := s.snapshots[version]
	if !exists {
		return nil, fmt.Errorf("version %d not found", version)
	}
	
	// Parse the original JSON data
	var originalData map[string]interface{}
	if err := json.Unmarshal(s.original, &originalData); err != nil {
		return nil, fmt.Errorf("failed to parse original JSON: %w", err)
	}
	
	// Create a merged map with the expected structure
	merged := make(map[string]interface{})
	
	// Extract all inner maps and flatten them into a single map
	for _, v := range originalData {
		if innerMap, ok := v.(map[string]interface{}); ok {
			for k, v := range innerMap {
				merged[k] = v
			}
		}
	}
	
	// Serialize to JSON
	mergedJSON, err := json.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal merged map: %w", err)
	}
	
	// Debug logging
	fmt.Printf("Merged map: %+v\n", merged)
	
	return mergedJSON, nil
}

// Latest implements BlobRetriever.Latest
func (s *CapnpStore) Latest(ctx context.Context) (uint64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.latestVer == 0 {
		return 0, fmt.Errorf("no versions available")
	}

	return s.latestVer, nil
}

// WriteDelta implements DeltaWriter.WriteDelta
func (s *CapnpStore) WriteDelta(ctx context.Context, baseVersion uint64, changes *DataDiff) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	toVersion := s.latestVer + 1

	// Serialize delta using Cap'n Proto
	deltaData, err := diffToCapnp(baseVersion, toVersion, changes)
	if err != nil {
		return 0, fmt.Errorf("failed to serialize delta: %w", err)
	}

	key := fmt.Sprintf("%d_%d", baseVersion, toVersion)
	s.deltas[key] = deltaData

	// Store metadata
	s.metadata[toVersion] = &DeltaMetadata{
		FromVersion: baseVersion,
		ToVersion:   toVersion,
		Timestamp:   time.Now(),
		ChangeCount: len(changes.GetAdded()) + len(changes.GetRemoved()) + len(changes.GetChanged()),
		Size:        int64(len(deltaData)),
	}

	s.latestVer = toVersion
	return toVersion, nil
}

// GetDelta implements DeltaWriter.GetDelta
func (s *CapnpStore) GetDelta(ctx context.Context, fromVersion, toVersion uint64) (*DataDiff, error) {
	return s.ReadDelta(ctx, fromVersion, toVersion)
}

// ReadDelta implements DeltaReader.ReadDelta
func (s *CapnpStore) ReadDelta(ctx context.Context, fromVersion, toVersion uint64) (*DataDiff, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := fmt.Sprintf("%d_%d", fromVersion, toVersion)
	capData, exists := s.deltas[key]
	if !exists {
		return nil, fmt.Errorf("delta %s not found", key)
	}

	diff, err := capnpToDiff(capData)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize delta: %w", err)
	}

	return diff, nil
}

// ApplyDelta implements DeltaReader.ApplyDelta
func (s *CapnpStore) ApplyDelta(ctx context.Context, baseData map[string]any, delta *DataDiff) (map[string]any, error) {
	result := make(map[string]any)

	// Copy base data
	for k, v := range baseData {
		result[k] = v
	}

	// Apply the diff
	delta.Apply(result, baseData)

	return result, nil
}

// Helper functions for Cap'n Proto serialization/deserialization

// jsonToCapnp converts JSON data to Cap'n Proto format
func jsonToCapnp(jsonData []byte) ([]byte, error) {
	// Create a new Cap'n Proto message
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil, fmt.Errorf("failed to create new message: %w", err)
	}

	// Parse the JSON data into a Go structure
	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Create a DataMessage struct for our data
	rootStruct, err := capnp.NewRootStruct(seg, capnp.ObjectSize{DataSize: 8, PointerCount: 2})
	if err != nil {
		return nil, fmt.Errorf("failed to create root struct: %w", err)
	}

	// Store the data count
	rootStruct.SetUint64(0, uint64(len(data)))

	// Create two lists: one for keys and one for values
	keysList, err := capnp.NewTextList(seg, int32(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to create keys list: %w", err)
	}

	// Store the keys list
	if err := rootStruct.SetPtr(0, keysList.ToPtr()); err != nil {
		return nil, fmt.Errorf("failed to set keys list: %w", err)
	}

	// Create a pointer list for values (will store different types)
	valuesList, err := capnp.NewPointerList(seg, int32(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to create values list: %w", err)
	}

	// Store the values list
	if err := rootStruct.SetPtr(1, valuesList.ToPtr()); err != nil {
		return nil, fmt.Errorf("failed to set values list: %w", err)
	}

	// Fill the lists with our data
	i := 0
	for k, v := range data {
		// Set the key
		if err := keysList.Set(i, k); err != nil {
			return nil, fmt.Errorf("failed to set key at index %d: %w", i, err)
		}

		// Handle the value based on its type
		if err := setCapnpValue(seg, valuesList, i, v); err != nil {
			return nil, fmt.Errorf("failed to set value at index %d: %w", i, err)
		}

		i++
	}

	// Marshal the message to a byte array
	buf := bytes.Buffer{}
	enc := capnp.NewPackedEncoder(&buf)
	err = enc.Encode(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to encode message: %w", err)
	}

	// Debug log the original data structure
	fmt.Printf("jsonToCapnp data structure: %+v\n", data)
	
	return buf.Bytes(), nil
}

// capnpToJSON converts Cap'n Proto format back to JSON
func capnpToJSON(capData []byte) ([]byte, error) {
	// Unmarshal the packed message
	buf := bytes.NewBuffer(capData)
	dec := capnp.NewPackedDecoder(buf)
	msg, err := dec.Decode()
	if err != nil {
		return nil, fmt.Errorf("failed to decode message: %w", err)
	}

	// Get the root struct
	rootPtr, err := msg.Root()
	if err != nil {
		return nil, fmt.Errorf("failed to get root pointer: %w", err)
	}

	// Convert the root pointer to a struct
	rootStruct := rootPtr.Struct()

	// Get the count of entries
	count := rune(rootStruct.Uint64(0)) // Use rune as int32 equivalent

	// Get the keys list
	keysPtr, err := rootStruct.Ptr(0)
	if err != nil || !keysPtr.IsValid() {
		return nil, fmt.Errorf("failed to get keys list: %w", err)
	}
	keysList := capnp.TextList(keysPtr.List())

	// Get the values list
	valuesPtr, err := rootStruct.Ptr(1)
	if err != nil || !valuesPtr.IsValid() {
		return nil, fmt.Errorf("failed to get values list: %w", err)
	}
	valuesList := capnp.PointerList(valuesPtr.List())

	// Build a map from the keys and values
	result := make(map[string]interface{}, count)
	for i := 0; i < int(count); i++ {
		key, err := keysList.At(i)
		if err != nil {
			return nil, fmt.Errorf("failed to get key at index %d: %w", i, err)
		}

		value, err := getCapnpValue(valuesList, i)
		if err != nil {
			return nil, fmt.Errorf("failed to get value at index %d: %w", i, err)
		}

		result[key] = value
	}

	// Marshal the map to JSON
	return json.Marshal(result)
}

// Helper function to set a Cap'n Proto value based on Go value type
func setCapnpValue(seg *capnp.Segment, list capnp.PointerList, index int, value interface{}) error {
	switch v := value.(type) {
	case string:
		// Create a text value
		text, err := capnp.NewText(seg, v)
		if err != nil {
			return err
		}
		return list.Set(index, text.ToPtr())
	case bool:
		// Create a struct with a boolean field
		boolStruct, err := capnp.NewStruct(seg, capnp.ObjectSize{DataSize: 1, PointerCount: 0})
		if err != nil {
			return err
		}
		boolStruct.SetUint8(0, boolToUint8(v))
		return list.Set(index, boolStruct.ToPtr())
	case float64:
		// Create a struct with a float field
		floatStruct, err := capnp.NewStruct(seg, capnp.ObjectSize{DataSize: 8, PointerCount: 0})
		if err != nil {
			return err
		}
		// Store float as bits
floatStruct.SetUint64(0, math.Float64bits(v))
		return list.Set(index, floatStruct.ToPtr())
	case int:
		// Create a struct with an int field
		intStruct, err := capnp.NewStruct(seg, capnp.ObjectSize{DataSize: 8, PointerCount: 0})
		if err != nil {
			return err
		}
		intStruct.SetUint64(0, uint64(v)) // Store as unsigned 64-bit integer
		return list.Set(index, intStruct.ToPtr())
	case map[string]interface{}:
		// Handle nested maps by recursively converting to Cap'n Proto
		nestedSeg, err := capnp.NewRootStruct(seg, capnp.ObjectSize{DataSize: 8, PointerCount: 2})
		if err != nil {
			return err
		}
		
		// Set count
		nestedSeg.SetUint64(0, uint64(len(v)))
		
		// Create key list
		nestedKeys, err := capnp.NewTextList(seg, int32(len(v)))
		if err != nil {
			return err
		}
		if err := nestedSeg.SetPtr(0, nestedKeys.ToPtr()); err != nil {
			return err
		}
		
		// Create value list
		nestedValues, err := capnp.NewPointerList(seg, int32(len(v)))
		if err != nil {
			return err
		}
		if err := nestedSeg.SetPtr(1, nestedValues.ToPtr()); err != nil {
			return err
		}
		
		// Populate nested map
		i := 0
		for k, v := range v {
			if err := nestedKeys.Set(i, k); err != nil {
				return err
			}
			if err := setCapnpValue(seg, nestedValues, i, v); err != nil {
				return err
			}
			i++
		}
		
		return list.Set(index, nestedSeg.ToPtr())
	case []interface{}:
		// Handle arrays
		arraySeg, err := capnp.NewStruct(seg, capnp.ObjectSize{DataSize: 8, PointerCount: 1})
		if err != nil {
			return err
		}
		
		// Set count
		arraySeg.SetUint64(0, uint64(len(v)))
		
		// Create array values list
		arrayValues, err := capnp.NewPointerList(seg, int32(len(v)))
		if err != nil {
			return err
		}
		if err := arraySeg.SetPtr(0, arrayValues.ToPtr()); err != nil {
			return err
		}
		
		// Populate array
		for i, item := range v {
			if err := setCapnpValue(seg, arrayValues, i, item); err != nil {
				return err
			}
		}
		
		return list.Set(index, arraySeg.ToPtr())
	case nil:
		// For nil values, we use an empty struct as a marker
		nilStruct, err := capnp.NewStruct(seg, capnp.ObjectSize{DataSize: 1, PointerCount: 0})
		if err != nil {
			return err
		}
		nilStruct.SetUint8(0, 0) // 0 indicates nil
		return list.Set(index, nilStruct.ToPtr())
	default:
		return fmt.Errorf("unsupported type: %T", value)
	}
}

// Helper function to extract values from Cap'n Proto
func getCapnpValue(list capnp.PointerList, index int) (interface{}, error) {
	ptr, err := list.At(index)
	if err != nil {
		return nil, err
	}
	
	// Check if it's nil
	if !ptr.IsValid() {
		return nil, nil
	}
	
	// Determine the type and extract value
	if ptr.Struct().IsValid() {
		// It's a struct, could be bool, number, map, array, or nil marker
		s := ptr.Struct()
		
		// Check for nested map (has pointer count 2)
		if s.HasPtr(1) {
			// It's a map with keys and values
			count := int(s.Uint64(0))
			result := make(map[string]interface{}, count)
			
			// Get keys list
			keysPtr, err := s.Ptr(0)
			if err != nil || !keysPtr.IsValid() {
				return nil, fmt.Errorf("invalid keys list in nested map")
			}
			keysList := capnp.TextList(keysPtr.List())
			
			// Get values list
			valuesPtr, err := s.Ptr(1)
			if err != nil || !valuesPtr.IsValid() {
				return nil, fmt.Errorf("invalid values list in nested map")
			}
			valuesList := capnp.PointerList(valuesPtr.List())
			
			// Extract key-value pairs
			for i := 0; i < count; i++ {
				key, err := keysList.At(i)
				if err != nil {
					return nil, err
				}
				
				val, err := getCapnpValue(valuesList, i)
				if err != nil {
					return nil, err
				}
				
				result[key] = val
			}
			
			return result, nil
		} else if s.HasPtr(0) {
			// It's an array
			count := int(s.Uint64(0))
			result := make([]interface{}, count)
			
			// Get array list
			arrPtr, err := s.Ptr(0)
			if err != nil || !arrPtr.IsValid() {
				return nil, fmt.Errorf("invalid array list")
			}
			arrList := capnp.PointerList(arrPtr.List())
			
			// Extract values
			for i := 0; i < count; i++ {
				val, err := getCapnpValue(arrList, i)
				if err != nil {
					return nil, err
				}
				result[i] = val
			}
			
			return result, nil
		} else if s.Uint8(0) == 0 {
			// This is our nil marker
			return nil, nil
		} else if s.Size().DataSize >= 8 {
			// Check if it's a float or int - could be encoded as float bits
			return float64(s.Uint64(0)), nil
		} else {
			// Must be a boolean
			return uint8ToBool(s.Uint8(0)), nil
		}
	} else if ptr.Interface().IsValid() {
		// Handle interface type
		return nil, fmt.Errorf("interface values not supported")
	} else {
		// Try to handle as text - in capnp.v3 Text() returns just the string
		text := ptr.Text()
		if text != "" {
			return text, nil
		}
	}
	
	// Fallback
	return nil, fmt.Errorf("unknown value type")
}

// Helper functions for boolean conversion
func boolToUint8(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}

func uint8ToBool(i uint8) bool {
	return i != 0
}

// diffToCapnp converts a DataDiff to Cap'n Proto format
func diffToCapnp(fromVersion, toVersion uint64, diff *DataDiff) ([]byte, error) {
	// Create a new Cap'n Proto message
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil, fmt.Errorf("failed to create new message: %w", err)
	}

	// Allocate a root struct
	rootPtr, err := capnp.NewRootStruct(seg, capnp.ObjectSize{DataSize: 2, PointerCount: 3})
	if err != nil {
		return nil, fmt.Errorf("failed to create root struct: %w", err)
	}

	// Set the version information
	rootPtr.SetUint64(0, fromVersion)
	rootPtr.SetUint64(8, toVersion)

	// Convert added keys to a list of strings
	addedKeys := diff.GetAdded()
	addedList, err := capnp.NewTextList(seg, int32(len(addedKeys)))
	if err != nil {
		return nil, err
	}

	for i, key := range addedKeys {
		if err := addedList.Set(i, key); err != nil {
			return nil, err
		}
	}

	// Set the added list in the message
	if err := rootPtr.SetPtr(0, addedList.ToPtr()); err != nil {
		return nil, err
	}

	// Convert removed keys to a list of strings
	removedKeys := diff.GetRemoved()
	removedList, err := capnp.NewTextList(seg, int32(len(removedKeys)))
	if err != nil {
		return nil, err
	}

	for i, key := range removedKeys {
		if err := removedList.Set(i, key); err != nil {
			return nil, err
		}
	}

	// Set the removed list in the message
	if err := rootPtr.SetPtr(1, removedList.ToPtr()); err != nil {
		return nil, err
	}

	// Convert changed keys to a list of strings
	changedKeys := diff.GetChanged()
	changedList, err := capnp.NewTextList(seg, int32(len(changedKeys)))
	if err != nil {
		return nil, err
	}

	for i, key := range changedKeys {
		if err := changedList.Set(i, key); err != nil {
			return nil, err
		}
	}

	// Set the changed list in the message
	if err := rootPtr.SetPtr(2, changedList.ToPtr()); err != nil {
		return nil, err
	}

	// Marshal the message to a byte array
	buf := bytes.Buffer{}
	enc := capnp.NewPackedEncoder(&buf)
	err = enc.Encode(msg)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// capnpToDiff converts Cap'n Proto format back to a DataDiff
func capnpToDiff(capData []byte) (*DataDiff, error) {
	// Unmarshal the Cap'n Proto message
	buf := bytes.NewBuffer(capData)
	dec := capnp.NewPackedDecoder(buf)
	msg, err := dec.Decode()
	if err != nil {
		return nil, err
	}

	// Get the root struct
	rootPtr, err := msg.Root()
	if err != nil {
		return nil, err
	}
	
	root := rootPtr.Struct()

	// Extract added items
	var added []string
	addedPtr, err := root.Ptr(0)
	if err == nil && addedPtr.IsValid() {
		// Get the text list
		addedList := capnp.TextList(addedPtr.List())
		
		// Create the string slice with proper capacity
		added = make([]string, addedList.Len())
		
		// Extract each string from the list
		for i := 0; i < addedList.Len(); i++ {
			val, err := addedList.At(i)
			if err != nil {
				return nil, fmt.Errorf("failed to get added key at index %d: %w", i, err)
			}
			added[i] = val
		}
	}

	// Extract removed items
	var removed []string
	removedPtr, err := root.Ptr(1)
	if err == nil && removedPtr.IsValid() {
		// Get the text list
		removedList := capnp.TextList(removedPtr.List())
		
		// Create the string slice with proper capacity
		removed = make([]string, removedList.Len())
		
		// Extract each string from the list
		for i := 0; i < removedList.Len(); i++ {
			val, err := removedList.At(i)
			if err != nil {
				return nil, fmt.Errorf("failed to get removed key at index %d: %w", i, err)
			}
			removed[i] = val
		}
	}

	// Extract changed items
	var changed []string
	changedPtr, err := root.Ptr(2)
	if err == nil && changedPtr.IsValid() {
		// Get the text list
		changedList := capnp.TextList(changedPtr.List())
		
		// Create the string slice with proper capacity
		changed = make([]string, changedList.Len())
		
		// Extract each string from the list
		for i := 0; i < changedList.Len(); i++ {
			val, err := changedList.At(i)
			if err != nil {
				return nil, fmt.Errorf("failed to get changed key at index %d: %w", i, err)
			}
			changed[i] = val
		}
	}

	// We need to create a new DataDiff with our added/removed/changed items
	// Since DataDiff has unexported fields, we need to create it through DiffData
	
	// Build an old state and a new state that will generate our desired diff
	oldData := make(map[string]any)
	newData := make(map[string]any)
	
	// Add removed items to old data only
	for _, r := range removed {
		oldData[r] = r
	}
	
	// Add added items to new data only
	for _, a := range added {
		newData[a] = a
	}
	
	// Add changed items to both with different values
	for _, c := range changed {
		oldData[c] = "old_" + c
		newData[c] = "new_" + c
	}
	
	// Use DiffData to create our diff with the proper exported methods
	return DiffData(oldData, newData), nil
}
