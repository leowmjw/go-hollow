package hollow

import (
	"context"
	"fmt"
)

// SerializationFormat represents the serialization format to use
type SerializationFormat int

const (
	// JSONFormat is the default JSON serialization format
	JSONFormat SerializationFormat = iota
	
	// CapnProtoFormat uses Cap'n Proto for serialization
	CapnProtoFormat
)

// ProducerOptions contains configuration options for a Producer
type ProducerOptions struct {
	// Format specifies the serialization format to use (default: JSONFormat)
	Format SerializationFormat
	
	// BlobStager is the stager to use. If nil, an in-memory stager will be created
	// based on the Format option.
	BlobStager BlobStager
}

// ConsumerOptions contains configuration options for a Consumer
type ConsumerOptions struct {
	// Format specifies the serialization format to use (default: JSONFormat)
	Format SerializationFormat
	
	// BlobRetriever is the retriever to use. If nil, an in-memory retriever will be created
	// based on the Format option.
	BlobRetriever BlobRetriever
}

// NewProducerWithOptions creates a new Producer with the specified options
func NewProducerWithOptions(opts ProducerOptions) *Producer {
	var stager BlobStager
	
	if opts.BlobStager != nil {
		stager = opts.BlobStager
	} else {
		// Create a stager based on the format
		switch opts.Format {
		case CapnProtoFormat:
			stager = NewCapnpStore()
		default: // JSONFormat
			stager = NewDeltaAwareMemBlobStore()
		}
	}
	
	p := &Producer{
		stager: stager,
	}
	
	// If using Cap'n Proto format, use a Cap'n Proto store
	if opts.Format == CapnProtoFormat {
		// Make sure we're using a Cap'n Proto store
		if _, ok := stager.(*CapnpStore); !ok {
			// If not already a CapnpStore, create one
			stager = NewCapnpStore()
		}
	}
	
	return p
}

// NewConsumerWithOptions creates a new Consumer with the specified options
func NewConsumerWithOptions(opts ConsumerOptions) *Consumer {
	var retriever BlobRetriever
	
	if opts.BlobRetriever != nil {
		retriever = opts.BlobRetriever
	} else {
		// Create a retriever based on the format
		switch opts.Format {
		case CapnProtoFormat:
			retriever = NewCapnpStore()
		default: // JSONFormat
			retriever = NewDeltaAwareMemBlobStore()
		}
	}
	
	// Create a consumer with the given retriever
	consumer := &Consumer{
		retriever: retriever,
	}
	
	// Initialize state
	rs := &readState{
		data: make(map[string]any),
	}
	consumer.state.Store(rs)
	
	return consumer
}

// NewDeltaProducerWithOptions creates a new DeltaProducer with the specified options
func NewDeltaProducerWithOptions(opts ProducerOptions) (*DeltaProducer, error) {
	var store DeltaAwareBlobStore
	
	if opts.BlobStager != nil {
		// Check if the provided stager is also a DeltaAwareBlobStore
		deltaStore, ok := opts.BlobStager.(DeltaAwareBlobStore)
		if !ok {
			return nil, fmt.Errorf("stager does not implement DeltaAwareBlobStore")
		}
		store = deltaStore
	} else {
		// Create a store based on the format
		switch opts.Format {
		case CapnProtoFormat:
			store = NewCapnpStore()
		default: // JSONFormat
			store = NewDeltaAwareMemBlobStore()
		}
	}
	
	// NewDeltaProducer accepts a DeltaAwareBlobStore as its first argument
	dp := NewDeltaProducer()
	// Set the store directly since we can't use it as a ProducerOpt
	dp.stager = store
	return dp, nil
}

// NewDeltaConsumerWithOptions creates a new DeltaConsumer with the specified options
func NewDeltaConsumerWithOptions(ctx context.Context, opts ConsumerOptions) (*DeltaConsumer, error) {
	var store DeltaAwareBlobStore
	
	if opts.BlobRetriever != nil {
		// Check if the provided retriever is also a DeltaAwareBlobStore
		deltaStore, ok := opts.BlobRetriever.(DeltaAwareBlobStore)
		if !ok {
			return nil, fmt.Errorf("retriever does not implement DeltaAwareBlobStore")
		}
		store = deltaStore
	} else {
		// Create a store based on the format
		switch opts.Format {
		case CapnProtoFormat:
			store = NewCapnpStore()
		default: // JSONFormat
			store = NewDeltaAwareMemBlobStore()
		}
	}
	
	version, err := store.Latest(ctx)
	if err != nil {
		return nil, err
	}
	
	// NewDeltaConsumer doesn't accept DeltaAwareBlobStore directly
	dc := NewDeltaConsumer()
	// Set the store and version directly
	dc.retriever = store
	dc.version.Store(version)
	return dc, nil
}

// NewCapnProtoProducer creates a new Producer using Cap'n Proto serialization
func NewCapnProtoProducer() *Producer {
	return NewProducerWithOptions(ProducerOptions{
		Format: CapnProtoFormat,
	})
}

// NewCapnProtoConsumer creates a new Consumer using Cap'n Proto serialization
func NewCapnProtoConsumer() *Consumer {
	return NewConsumerWithOptions(ConsumerOptions{
		Format: CapnProtoFormat,
	})
}

// NewCapnProtoDeltaProducer creates a new DeltaProducer using Cap'n Proto serialization
func NewCapnProtoDeltaProducer() (*DeltaProducer, error) {
	return NewDeltaProducerWithOptions(ProducerOptions{
		Format: CapnProtoFormat,
	})
}

// NewCapnProtoDeltaConsumer creates a new DeltaConsumer using Cap'n Proto serialization
func NewCapnProtoDeltaConsumer(ctx context.Context) (*DeltaConsumer, error) {
	return NewDeltaConsumerWithOptions(ctx, ConsumerOptions{
		Format: CapnProtoFormat,
	})
}
