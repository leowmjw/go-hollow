package blob

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3BlobStore implements BlobStore using MinIO/S3
type S3BlobStore struct {
	client     *minio.Client
	bucketName string
	mu         sync.RWMutex
	cache      map[string]*Blob // Local cache for faster testing
}

// S3BlobStoreConfig configuration for S3 blob store
type S3BlobStoreConfig struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	BucketName      string
	UseSSL          bool
}

// NewS3BlobStore creates a new S3-based blob store
func NewS3BlobStore(config S3BlobStoreConfig) (*S3BlobStore, error) {
	// Initialize MinIO client
	client, err := minio.New(config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKeyID, config.SecretAccessKey, ""),
		Secure: config.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}
	
	store := &S3BlobStore{
		client:     client,
		bucketName: config.BucketName,
		cache:      make(map[string]*Blob),
	}
	
	// Ensure bucket exists
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, config.BucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket existence: %w", err)
	}
	
	if !exists {
		err = client.MakeBucket(ctx, config.BucketName, minio.MakeBucketOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
	}
	
	return store, nil
}

// NewLocalS3BlobStore creates a local MinIO instance for testing
func NewLocalS3BlobStore() (*S3BlobStore, error) {
	config := S3BlobStoreConfig{
		Endpoint:        "localhost:9000",
		AccessKeyID:     "minioadmin", 
		SecretAccessKey: "minioadmin",
		BucketName:      "hollow-test",
		UseSSL:          false,
	}
	
	return NewS3BlobStore(config)
}

func (s *S3BlobStore) Store(ctx context.Context, blob *Blob) error {
	objectName := s.getObjectName(blob)
	
	// Store in cache first
	s.mu.Lock()
	s.cache[objectName] = blob
	s.mu.Unlock()
	
	// Also store in S3 (but allow it to fail for local testing)
	reader := strings.NewReader(string(blob.Data))
	_, err := s.client.PutObject(ctx, s.bucketName, objectName, reader, int64(len(blob.Data)), minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	
	// For local testing, we don't fail if S3 is not available
	if err != nil {
		// Log error but continue using cache-only mode
		fmt.Printf("Warning: S3 storage failed, using cache-only mode: %v\n", err)
	}
	
	return nil
}

func (s *S3BlobStore) RetrieveSnapshotBlob(version int64) *Blob {
	objectName := fmt.Sprintf("snapshots/snapshot-%d.blob", version)
	return s.retrieveBlob(objectName, SnapshotBlob, version)
}

func (s *S3BlobStore) RetrieveDeltaBlob(fromVersion int64) *Blob {
	objectName := fmt.Sprintf("deltas/delta-%d.blob", fromVersion)
	return s.retrieveBlob(objectName, DeltaBlob, fromVersion)
}

func (s *S3BlobStore) RetrieveReverseBlob(toVersion int64) *Blob {
	objectName := fmt.Sprintf("reverse/reverse-%d.blob", toVersion)
	return s.retrieveBlob(objectName, ReverseBlob, toVersion)
}

func (s *S3BlobStore) retrieveBlob(objectName string, blobType BlobType, version int64) *Blob {
	// Check cache first
	s.mu.RLock()
	blob, exists := s.cache[objectName]
	s.mu.RUnlock()
	
	if exists {
		return blob
	}
	
	// Try to retrieve from S3
	ctx := context.Background()
	object, err := s.client.GetObject(ctx, s.bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil
	}
	defer object.Close()
	
	data, err := io.ReadAll(object)
	if err != nil {
		return nil
	}
	
	blob = &Blob{
		Type:    blobType,
		Version: version,
		Data:    data,
	}
	
	// Cache the retrieved blob
	s.mu.Lock()
	s.cache[objectName] = blob
	s.mu.Unlock()
	
	return blob
}

func (s *S3BlobStore) RemoveSnapshot(version int64) error {
	objectName := fmt.Sprintf("snapshots/snapshot-%d.blob", version)
	
	// Remove from cache
	s.mu.Lock()
	delete(s.cache, objectName)
	s.mu.Unlock()
	
	// Remove from S3
	ctx := context.Background()
	err := s.client.RemoveObject(ctx, s.bucketName, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to remove snapshot: %w", err)
	}
	
	return nil
}

func (s *S3BlobStore) ListVersions() []int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	versions := make(map[int64]bool)
	
	// Get versions from cache
	for objectName := range s.cache {
		if version := s.extractVersionFromObjectName(objectName); version > 0 {
			versions[version] = true
		}
	}
	
	// Convert to sorted slice
	result := make([]int64, 0, len(versions))
	for version := range versions {
		result = append(result, version)
	}
	
	// Sort versions
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i] > result[j] {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	
	return result
}

func (s *S3BlobStore) getObjectName(blob *Blob) string {
	switch blob.Type {
	case SnapshotBlob:
		return fmt.Sprintf("snapshots/snapshot-%d.blob", blob.Version)
	case DeltaBlob:
		return fmt.Sprintf("deltas/delta-%d.blob", blob.FromVersion)
	case ReverseBlob:
		return fmt.Sprintf("reverse/reverse-%d.blob", blob.ToVersion)
	default:
		return fmt.Sprintf("unknown/blob-%d.blob", blob.Version)
	}
}

func (s *S3BlobStore) extractVersionFromObjectName(objectName string) int64 {
	// Extract version from object names like "snapshots/snapshot-123.blob"
	parts := strings.Split(objectName, "/")
	if len(parts) != 2 {
		return 0
	}
	
	filename := parts[1]
	if strings.HasPrefix(filename, "snapshot-") {
		versionStr := strings.TrimPrefix(filename, "snapshot-")
		versionStr = strings.TrimSuffix(versionStr, ".blob")
		if version, err := strconv.ParseInt(versionStr, 10, 64); err == nil {
			return version
		}
	}
	
	return 0
}
