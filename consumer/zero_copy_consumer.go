// Zero-copy consumer extensions that provide direct access to Cap'n Proto data
package consumer

import (
	"context"
	"fmt"
	"strconv"

	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/internal"
)

// ZeroCopyConsumer extends the regular consumer with zero-copy data access capabilities
type ZeroCopyConsumer struct {
	*Consumer // Embed the regular consumer
	
	// Zero-copy specific state
	zeroCopyViews map[int64]internal.ZeroCopyView
	serializer    internal.Serializer
}

// ZeroCopyConsumerOption configures a ZeroCopyConsumer
type ZeroCopyConsumerOption func(*ZeroCopyConsumer)

func WithZeroCopySerializer(serializer internal.Serializer) ZeroCopyConsumerOption {
	return func(c *ZeroCopyConsumer) {
		c.serializer = serializer
	}
}

func WithZeroCopySerializationMode(mode internal.SerializationMode) ZeroCopyConsumerOption {
	return func(c *ZeroCopyConsumer) {
		factory := internal.NewSerializerFactory(mode)
		c.serializer = factory.CreateSerializer()
	}
}

// NewZeroCopyConsumer creates a consumer with zero-copy capabilities
func NewZeroCopyConsumer(options ...ConsumerOption) *ZeroCopyConsumer {
	// Create regular consumer first
	regularConsumer := NewConsumer(options...)
	
	// Wrap it with zero-copy capabilities
	zc := &ZeroCopyConsumer{
		Consumer:      regularConsumer,
		zeroCopyViews: make(map[int64]internal.ZeroCopyView),
	}
	
	return zc
}

// NewZeroCopyConsumerWithOptions creates a consumer with both regular and zero-copy options
func NewZeroCopyConsumerWithOptions(consumerOptions []ConsumerOption, zeroCopyOptions []ZeroCopyConsumerOption) *ZeroCopyConsumer {
	zc := NewZeroCopyConsumer(consumerOptions...)
	
	// Apply zero-copy specific options
	for _, opt := range zeroCopyOptions {
		opt(zc)
	}
	
	return zc
}

// TriggerRefreshToWithZeroCopy refreshes to a specific version with zero-copy support
func (c *ZeroCopyConsumer) TriggerRefreshToWithZeroCopy(ctx context.Context, version int64) error {
	// First, perform regular refresh
	err := c.Consumer.TriggerRefreshTo(ctx, version)
	if err != nil {
		return fmt.Errorf("regular refresh failed: %w", err)
	}
	
	// Then, create zero-copy view if possible
	err = c.createZeroCopyViewForVersion(ctx, version)
	if err != nil {
		// Zero-copy view creation failed, but regular refresh succeeded
		// Log the error but don't fail the operation
		// This allows graceful fallback to traditional access
		fmt.Printf("Zero-copy view creation failed for version %d: %v\n", version, err)
		return nil
	}
	
	return nil
}

// GetZeroCopyView returns a zero-copy view for the current version
func (c *ZeroCopyConsumer) GetZeroCopyView() (internal.ZeroCopyView, bool) {
	currentVersion := c.Consumer.GetCurrentVersion()
	view, exists := c.zeroCopyViews[currentVersion]
	return view, exists
}

// GetZeroCopyViewForVersion returns a zero-copy view for a specific version
func (c *ZeroCopyConsumer) GetZeroCopyViewForVersion(version int64) (internal.ZeroCopyView, bool) {
	view, exists := c.zeroCopyViews[version]
	return view, exists
}

// HasZeroCopySupport returns true if the current data supports zero-copy access
func (c *ZeroCopyConsumer) HasZeroCopySupport() bool {
	_, exists := c.GetZeroCopyView()
	return exists
}

// GetDataWithZeroCopyPreference returns data using zero-copy if available, traditional otherwise
func (c *ZeroCopyConsumer) GetDataWithZeroCopyPreference() map[string][]interface{} {
	// Try zero-copy first
	if view, exists := c.GetZeroCopyView(); exists {
		// Convert zero-copy view to traditional format for API compatibility
		return c.convertZeroCopyViewToTraditionalFormat(view)
	}
	
	// Fallback to traditional access
	readStateEngine := c.Consumer.GetStateEngine()
	if readStateEngine.GetCurrentVersion() > 0 {
		// For demo purposes, return empty data since we can't access internal state directly
		// In a production implementation, this would expose the appropriate getter method
		return make(map[string][]interface{})
	}
	
	return make(map[string][]interface{})
}

// createZeroCopyViewForVersion attempts to create a zero-copy view for a specific version
func (c *ZeroCopyConsumer) createZeroCopyViewForVersion(ctx context.Context, version int64) error {
	// Get the blob for this version - try snapshot first, then delta
	var dataBlob *blob.Blob
	
	// Access retriever through getter method
	retriever := c.Consumer.GetRetriever()
	if retriever != nil {
		dataBlob = retriever.RetrieveSnapshotBlob(version)
		if dataBlob == nil {
			// Try delta blob if snapshot not found
			dataBlob = retriever.RetrieveDeltaBlob(version-1)
		}
	}
	if dataBlob == nil {
		return fmt.Errorf("no blob found for version %d", version)
	}

	// If we got an empty delta blob, fall back to the nearest snapshot.
	// Empty deltas are valid (no changes) but contain no Cap'n Proto payload,
	// which would cause EOF on zero-copy deserialization attempts.
	if dataBlob.Type == blob.DeltaBlob && len(dataBlob.Data) == 0 {
		// Find the nearest snapshot <= version
		if c.Consumer != nil && c.Consumer.GetRetriever() != nil {
			retriever := c.Consumer.GetRetriever()
			versions := retriever.ListVersions()
			
			// Find the highest version <= target version that has a snapshot
			var snapshot *blob.Blob
			for i := len(versions) - 1; i >= 0; i-- {
				v := versions[i]
				if v <= version {
					if snap := retriever.RetrieveSnapshotBlob(v); snap != nil {
						snapshot = snap
						break
					}
				}
			}
			
			if snapshot != nil {
				dataBlob = snapshot
			} else {
				return fmt.Errorf("empty delta for version %d and no prior snapshot available", version)
			}
		} else {
			return fmt.Errorf("empty delta for version %d and no retriever available", version)
		}
	}
	
	// Check if blob supports zero-copy serialization
	if !c.blobSupportsZeroCopy(dataBlob) {
		return fmt.Errorf("blob does not support zero-copy serialization")
	}
	
	// Deserialize using zero-copy
	serializer := c.getSerializer()
	deserializedData, err := serializer.Deserialize(ctx, dataBlob.Data)
	if err != nil {
		return fmt.Errorf("failed to deserialize blob: %w", err)
	}
	
	// Check if deserialized data provides zero-copy view
	zeroCopyView, hasZeroCopy := deserializedData.GetZeroCopyView()
	if !hasZeroCopy {
		return fmt.Errorf("deserialized data does not provide zero-copy view")
	}
	
	// Store the zero-copy view
	c.zeroCopyViews[version] = zeroCopyView
	
	return nil
}

// blobSupportsZeroCopy checks if a blob was serialized using zero-copy format
func (c *ZeroCopyConsumer) blobSupportsZeroCopy(blob *blob.Blob) bool {
	if blob.Metadata == nil {
		return false
	}
	
	serializationModeStr, exists := blob.Metadata["serialization_mode"]
	if !exists {
		return false
	}
	
	serializationMode, err := strconv.Atoi(serializationModeStr)
	if err != nil {
		return false
	}
	
	mode := internal.SerializationMode(serializationMode)
	return mode == internal.ZeroCopyMode || mode == internal.HybridMode
}

// getSerializer returns the appropriate serializer
func (c *ZeroCopyConsumer) getSerializer() internal.Serializer {
	if c.serializer != nil {
		return c.serializer
	}
	
	// Default to hybrid serializer that can handle both modes
	return internal.NewHybridSerializer(internal.ZeroCopyMode)
}

// convertZeroCopyViewToTraditionalFormat converts a zero-copy view to traditional map format
func (c *ZeroCopyConsumer) convertZeroCopyViewToTraditionalFormat(view internal.ZeroCopyView) map[string][]interface{} {
	// This is a simplified conversion for API compatibility
	// In a real implementation, this would traverse the Cap'n Proto message
	// and extract data into the traditional format expected by existing code
	
	result := make(map[string][]interface{})
	
	// For demo purposes, create placeholder data that indicates zero-copy access
	result["zero_copy_data"] = []interface{}{
		fmt.Sprintf("Zero-copy view with buffer size: %d bytes", len(view.GetByteBuffer())),
	}
	
	return result
}

// ZeroCopyDataAccessor provides type-safe access to specific Cap'n Proto types
type ZeroCopyDataAccessor struct {
	view internal.ZeroCopyView
}

// NewZeroCopyDataAccessor creates a new data accessor from a zero-copy view
func NewZeroCopyDataAccessor(view internal.ZeroCopyView) *ZeroCopyDataAccessor {
	return &ZeroCopyDataAccessor{view: view}
}

// GetRawBuffer returns direct access to the underlying Cap'n Proto buffer
func (a *ZeroCopyDataAccessor) GetRawBuffer() []byte {
	return a.view.GetByteBuffer()
}

// GetMessage returns the Cap'n Proto message for advanced operations
func (a *ZeroCopyDataAccessor) GetMessage() (*internal.ZeroCopyView, error) {
	return &a.view, nil
}

// GetBufferSize returns the size of the underlying buffer
func (a *ZeroCopyDataAccessor) GetBufferSize() int {
	return len(a.view.GetByteBuffer())
}

// ZeroCopyQueryEngine provides high-performance queries on zero-copy data
type ZeroCopyQueryEngine struct {
	view     internal.ZeroCopyView
	accessor *ZeroCopyDataAccessor
}

// NewZeroCopyQueryEngine creates a query engine for zero-copy data
func NewZeroCopyQueryEngine(view internal.ZeroCopyView) *ZeroCopyQueryEngine {
	return &ZeroCopyQueryEngine{
		view:     view,
		accessor: NewZeroCopyDataAccessor(view),
	}
}

// CountRecords returns the number of records without deserializing all data
func (q *ZeroCopyQueryEngine) CountRecords() (int, error) {
	// This would contain logic to count records directly from Cap'n Proto buffer
	// without full deserialization - just parsing the list headers
	
	// For demo purposes, return estimated count based on buffer size
	bufferSize := q.accessor.GetBufferSize()
	estimatedRecordSize := 64 // bytes per record estimate
	estimatedCount := bufferSize / estimatedRecordSize
	
	return estimatedCount, nil
}

// FindByOffset provides direct access to a record at a specific buffer offset
func (q *ZeroCopyQueryEngine) FindByOffset(offset int) ([]byte, error) {
	buffer := q.accessor.GetRawBuffer()
	
	if offset < 0 || offset >= len(buffer) {
		return nil, fmt.Errorf("offset %d out of range [0, %d)", offset, len(buffer))
	}
	
	// This would contain logic to parse a single record from the buffer
	// For demo purposes, return a slice of the buffer
	if offset+64 > len(buffer) {
		return buffer[offset:], nil
	}
	return buffer[offset : offset+64], nil
}

// ExtractField extracts a specific field from all records without full deserialization
func (q *ZeroCopyQueryEngine) ExtractField(fieldName string) ([]interface{}, error) {
	// This would contain logic to traverse the Cap'n Proto buffer and extract
	// only the specified field from all records, avoiding full deserialization
	
	// For demo purposes, return placeholder data
	result := []interface{}{
		fmt.Sprintf("Field '%s' extracted from zero-copy buffer", fieldName),
		fmt.Sprintf("Buffer size: %d bytes", q.accessor.GetBufferSize()),
	}
	
	return result, nil
}
