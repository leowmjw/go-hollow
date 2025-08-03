package producer

import (
	"context"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"sync"
	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/internal"
)

// Producer handles data production cycles
type Producer struct {
	writeEngine              *internal.WriteStateEngine
	blobStore                blob.BlobStore
	announcer                blob.Announcer
	validators               []internal.Validator
	singleProducerEnforcer   *internal.SingleProducerEnforcer
	typeResharding           bool
	targetMaxTypeShardSize   int
	numStatesBetweenSnapshots int
	// Mutex protects concurrent access to version and hash state
	mu                       sync.Mutex
	currentVersion           int64
	lastSnapshotVersion      int64
	lastDataHash             uint64
	// Zero-copy serialization support
	serializer               internal.Serializer
}

// ProducerOption configures a Producer
type ProducerOption func(*Producer)

func WithBlobStore(store blob.BlobStore) ProducerOption {
	return func(p *Producer) {
		p.blobStore = store
	}
}

func WithAnnouncer(announcer blob.Announcer) ProducerOption {
	return func(p *Producer) {
		p.announcer = announcer
	}
}

func WithValidator(validator internal.Validator) ProducerOption {
	return func(p *Producer) {
		p.validators = append(p.validators, validator)
	}
}

func WithSerializer(serializer internal.Serializer) ProducerOption {
	return func(p *Producer) {
		p.serializer = serializer
	}
}

func WithSerializationMode(mode internal.SerializationMode) ProducerOption {
	return func(p *Producer) {
		factory := internal.NewSerializerFactory(mode)
		p.serializer = factory.CreateSerializer()
	}
}

func WithSingleProducerEnforcer(enforcer *internal.SingleProducerEnforcer) ProducerOption {
	return func(p *Producer) {
		p.singleProducerEnforcer = enforcer
	}
}

func WithTypeResharding(enabled bool) ProducerOption {
	return func(p *Producer) {
		p.typeResharding = enabled
	}
}

func WithTargetMaxTypeShardSize(size int) ProducerOption {
	return func(p *Producer) {
		p.targetMaxTypeShardSize = size
	}
}

func WithNumStatesBetweenSnapshots(count int) ProducerOption {
	return func(p *Producer) {
		p.numStatesBetweenSnapshots = count
	}
}

// NewProducer creates a new Producer
func NewProducer(opts ...ProducerOption) *Producer {
	p := &Producer{
		writeEngine:               internal.NewWriteStateEngine(),
		validators:                make([]internal.Validator, 0),
		singleProducerEnforcer:    internal.NewSingleProducerEnforcer(),
		typeResharding:            false,
		targetMaxTypeShardSize:    1000,
		numStatesBetweenSnapshots: 5,
	}
	
	for _, opt := range opts {
		opt(p)
	}
	
	return p
}

// RunCycle executes a producer cycle and returns the new version
// For backward compatibility, this method ignores errors.
// Use RunCycleWithError for proper error handling.
func (p *Producer) RunCycle(ctx context.Context, populate func(*internal.WriteState)) int64 {
	version, _ := p.runCycleInternal(ctx, populate)
	return version
}

// RunCycleE executes a producer cycle and returns version and error
// This is the preferred method for new code that needs proper error handling.
func (p *Producer) RunCycleE(ctx context.Context, populate func(*internal.WriteState)) (int64, error) {
	return p.runCycleInternal(ctx, populate)
}

// RunCycleWithError executes a producer cycle and returns any validation errors
func (p *Producer) RunCycleWithError(ctx context.Context, populate func(*internal.WriteState)) error {
	_, err := p.runCycleInternal(ctx, populate)
	return err
}

func (p *Producer) runCycleInternal(ctx context.Context, populate func(*internal.WriteState)) (int64, error) {
	// Lock to protect concurrent access to producer state
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Check single producer enforcement
	if p.singleProducerEnforcer != nil && !p.singleProducerEnforcer.IsEnabled() {
		// Non-primary producer returns current version
		return p.currentVersion, nil
	}
	
	// Prepare for cycle
	p.writeEngine.PrepareForCycle()
	writeState := p.writeEngine.GetWriteState()
	
	// Populate data
	populate(writeState)
	
	// Check if cycle is empty
	if writeState.IsEmpty() {
		return 0, nil
	}
	
	// Calculate data hash to detect identical cycles
	dataHash := p.calculateDataHash(writeState.GetData())
	
	// Check if data is identical to previous cycle
	if p.currentVersion > 0 && p.lastDataHash != 0 && dataHash == p.lastDataHash {
		// Data is identical, return same version
		return p.currentVersion, nil
	}
	
	// Generate new version
	newVersion := p.currentVersion + 1
	
	// Perform type resharding if enabled
	if p.typeResharding {
		p.performResharding(writeState)
	}
	
	// Create read state for validation
	readState := internal.NewReadState(newVersion)
	
	// Run validation
	for _, validator := range p.validators {
		result := validator.Validate(readState)
		if result.Type == internal.ValidationFailed {
			// Rollback current cycle (don't change populated count from previous successful cycles)
			p.writeEngine.PrepareForCycle() // Reset current write state
			return 0, fmt.Errorf("validation failed: %s", result.Message)
		}
	}
	
	// Serialize and store blob
	err := p.storeBlob(ctx, newVersion, writeState)
	if err != nil {
		return 0, fmt.Errorf("failed to store blob: %w", err)
	}
	
	// Update version and hash
	p.currentVersion = newVersion
	p.lastDataHash = dataHash
	p.writeEngine.IncrementPopulatedCount()
	
	// Announce new version
	if p.announcer != nil {
		if err := p.announcer.Announce(newVersion); err != nil {
			// Log the error but don't fail the cycle since data is already stored
			// In a real implementation, this would use a proper logger
			fmt.Printf("Warning: failed to announce version %d: %v\n", newVersion, err)
		}
	}
	
	return newVersion, nil
}

func (p *Producer) calculateDataHash(data map[string][]interface{}) uint64 {
	h := md5.New()
	
	// Sort keys for consistent hashing
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	
	// Simple sort
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	
	for _, key := range keys {
		h.Write([]byte(key))
		values := data[key]
		binary.Write(h, binary.LittleEndian, int64(len(values)))
		for _, value := range values {
			h.Write([]byte(fmt.Sprintf("%v", value)))
		}
	}
	
	sum := h.Sum(nil)
	return binary.LittleEndian.Uint64(sum[:8])
}

func (p *Producer) performResharding(writeState *internal.WriteState) {
	data := writeState.GetData()
	
	for typeName, values := range data {
		if len(values) > p.targetMaxTypeShardSize {
			// Calculate new shard count
			newShardCount := (len(values) + p.targetMaxTypeShardSize - 1) / p.targetMaxTypeShardSize
			if newShardCount < 2 {
				newShardCount = 2
			}
			p.writeEngine.SetNumShards(typeName, newShardCount)
		}
	}
}

func (p *Producer) storeBlob(ctx context.Context, version int64, writeState *internal.WriteState) error {
	// Use pluggable serialization (supports both traditional and zero-copy modes)
	serializer := p.getSerializer()
	serializedData, err := serializer.Serialize(ctx, writeState)
	if err != nil {
		return fmt.Errorf("failed to serialize data: %w", err)
	}
	
	// Determine blob type
	var blobType blob.BlobType
	var fromVersion int64
	
	if p.shouldCreateSnapshot(version) {
		blobType = blob.SnapshotBlob
		p.lastSnapshotVersion = version
	} else {
		blobType = blob.DeltaBlob
		fromVersion = p.currentVersion
	}
	
	// Calculate hash from raw data for consistency across serialization modes
	data := writeState.GetData()
	
	// Create blob with serialized data
	blobData := &blob.Blob{
		Type:        blobType,
		Version:     version,
		FromVersion: fromVersion,
		ToVersion:   version,
		Data:        serializedData,
		Checksum:    p.calculateDataHash(data),
		// Add serialization mode metadata for consumer
		Metadata: map[string]string{
			"serialization_mode": fmt.Sprintf("%d", serializer.Mode()),
		},
	}
	
	// Store blob
	return p.blobStore.Store(ctx, blobData)
}

func (p *Producer) getSerializer() internal.Serializer {
	if p.serializer != nil {
		return p.serializer
	}
	// Default to traditional serialization for backward compatibility
	return internal.NewTraditionalSerializer()
}

func (p *Producer) shouldCreateSnapshot(version int64) bool {
	if p.lastSnapshotVersion == 0 {
		return true // First version is always a snapshot
	}
	
	shouldSnapshot := version-p.lastSnapshotVersion >= int64(p.numStatesBetweenSnapshots)
	return shouldSnapshot
}

// Restore restores producer state from a version
func (p *Producer) Restore(ctx context.Context, version int64, retriever blob.BlobRetriever) error {
	// Retrieve snapshot
	snapshotBlob := retriever.RetrieveSnapshotBlob(version)
	if snapshotBlob == nil {
		return fmt.Errorf("snapshot not found for version %d", version)
	}
	
	// Restore state (simplified)
	p.currentVersion = version
	
	return nil
}

// GetReadState returns a read state for the current version
func (p *Producer) GetReadState() *internal.ReadState {
	return internal.NewReadState(p.currentVersion)
}

// GetWriteEngine returns the write engine
func (p *Producer) GetWriteEngine() *internal.WriteStateEngine {
	return p.writeEngine
}

// TestRecord is a test data structure for type resharding tests
type TestRecord struct {
	ID    int
	Value int
}
