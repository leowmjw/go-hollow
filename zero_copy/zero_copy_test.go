package zerocopy

import (
	"context"
	"testing"

	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/generated/go/movie"
)

func TestZeroCopyReader_NewZeroCopyReader(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewInMemoryFeed()

	reader := NewZeroCopyReader(blobStore, announcer)

	if reader == nil {
		t.Fatal("expected reader to be created")
	}
	if reader.consumer == nil {
		t.Fatal("expected consumer to be initialized")
	}
	if reader.currentMsg != nil {
		t.Error("expected currentMsg to be nil initially")
	}
}

func TestZeroCopyReader_RefreshTo(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewInMemoryFeed()

	// Setup test data in blob store
	setupTestData(t, blobStore, announcer)

	reader := NewZeroCopyReader(blobStore, announcer)
	ctx := context.Background()

	err := reader.RefreshTo(ctx, 1)
	if err != nil {
		t.Fatalf("expected RefreshTo to succeed, got: %v", err)
	}

	if reader.currentMsg == nil {
		t.Error("expected currentMsg to be set after RefreshTo")
	}
}

func TestZeroCopyReader_GetMovies(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewInMemoryFeed()

	reader := NewZeroCopyReader(blobStore, announcer)

	// Test with no data loaded
	_, err := reader.GetMovies()
	if err == nil {
		t.Error("expected error when no data is loaded")
	}

	// Setup test data and test again
	setupTestData(t, blobStore, announcer)
	ctx := context.Background()
	err = reader.RefreshTo(ctx, 1)
	if err != nil {
		t.Fatalf("failed to refresh: %v", err)
	}

	movies, err := reader.GetMovies()
	if err != nil {
		t.Fatalf("expected GetMovies to succeed after refresh, got: %v", err)
	}

	if movies.Len() != 3 {
		t.Errorf("expected 3 movies, got %d", movies.Len())
	}
}

func TestZeroCopyReader_FindMovieById(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewInMemoryFeed()

	reader := NewZeroCopyReader(blobStore, announcer)
	ctx := context.Background()

	// Test with no data loaded
	_, found, err := reader.FindMovieById(1)
	if err == nil {
		t.Error("expected error when no data is loaded")
	}

	// Setup test data and test
	setupTestData(t, blobStore, announcer)
	err = reader.RefreshTo(ctx, 1)
	if err != nil {
		t.Fatalf("failed to refresh: %v", err)
	}

	// Test finding existing movie
	movie, found, err := reader.FindMovieById(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Error("expected to find movie with ID 1")
	}
	if movie.Id() != 1 {
		t.Errorf("expected movie ID 1, got %d", movie.Id())
	}

	// Test finding non-existent movie
	_, found, err = reader.FindMovieById(999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Error("expected not to find movie with ID 999")
	}
}

func TestZeroCopyReader_FilterMoviesByYear(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewInMemoryFeed()

	reader := NewZeroCopyReader(blobStore, announcer)
	ctx := context.Background()

	// Test with no data loaded
	_, err := reader.FilterMoviesByYear(1999)
	if err == nil {
		t.Error("expected error when no data is loaded")
	}

	// Setup test data and test
	setupTestData(t, blobStore, announcer)
	err = reader.RefreshTo(ctx, 1)
	if err != nil {
		t.Fatalf("failed to refresh: %v", err)
	}

	// Test filtering by existing year
	movies, err := reader.FilterMoviesByYear(1999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(movies) != 1 {
		t.Errorf("expected 1 movie from 1999, got %d", len(movies))
	}
	if movies[0].Year() != 1999 {
		t.Errorf("expected movie from 1999, got %d", movies[0].Year())
	}

	// Test filtering by non-existent year
	movies, err = reader.FilterMoviesByYear(1995)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(movies) != 0 {
		t.Errorf("expected 0 movies from 1995, got %d", len(movies))
	}
}

func TestZeroCopyReader_GetCurrentVersion(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewInMemoryFeed()

	reader := NewZeroCopyReader(blobStore, announcer)
	version := reader.GetCurrentVersion()

	// Should return 0 initially (from mock consumer)
	if version != 0 {
		t.Errorf("expected version 0, got %d", version)
	}
}

func TestZeroCopyWriter_NewZeroCopyWriter(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewInMemoryFeed()

	writer := NewZeroCopyWriter(blobStore, announcer)

	if writer == nil {
		t.Fatal("expected writer to be created")
	}
	if writer.producer == nil {
		t.Fatal("expected producer to be initialized")
	}
}

func TestZeroCopyWriter_WriteMovieDataset(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewInMemoryFeed()

	writer := NewZeroCopyWriter(blobStore, announcer)
	ctx := context.Background()

	// Test writing empty dataset
	movies := []MovieData{}
	version, err := writer.WriteMovieDataset(ctx, movies)
	if err != nil {
		t.Fatalf("unexpected error writing empty dataset: %v", err)
	}
	if version != 0 {
		t.Errorf("expected version 0 for empty dataset, got %d", version)
	}

	// Test writing dataset with movies
	movies = []MovieData{
		{
			ID:         1,
			Title:      "Test Movie",
			Year:       2023,
			RuntimeMin: 120,
			Genres:     []string{"Action", "Adventure"},
		},
		{
			ID:         2,
			Title:      "Another Movie",
			Year:       2024,
			RuntimeMin: 95,
			Genres:     []string{"Comedy"},
		},
	}

	version, err = writer.WriteMovieDataset(ctx, movies)
	if err != nil {
		t.Fatalf("unexpected error writing dataset: %v", err)
	}
	if version != 2 {
		t.Errorf("expected version 2, got %d", version)
	}
}

func TestZeroCopyWriter_WriteMovieDataset_EmptyGenres(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewInMemoryFeed()

	writer := NewZeroCopyWriter(blobStore, announcer)
	ctx := context.Background()

	// Test movie with no genres
	movies := []MovieData{
		{
			ID:         1,
			Title:      "Movie Without Genres",
			Year:       2023,
			RuntimeMin: 120,
			Genres:     []string{}, // Empty genres
		},
	}

	_, err := writer.WriteMovieDataset(ctx, movies)
	if err != nil {
		t.Fatalf("unexpected error writing dataset with empty genres: %v", err)
	}
}

func TestZeroCopyIterator_NewZeroCopyIterator(t *testing.T) {
	// Create a sample movie list for testing
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewInMemoryFeed()
	reader := NewZeroCopyReader(blobStore, announcer)
	ctx := context.Background()
	
	setupTestData(t, blobStore, announcer)
	err := reader.RefreshTo(ctx, 1)
	if err != nil {
		t.Fatalf("failed to refresh: %v", err)
	}

	movies, err := reader.GetMovies()
	if err != nil {
		t.Fatalf("failed to get movies: %v", err)
	}

	iterator := NewZeroCopyIterator(movies, 2)

	if iterator == nil {
		t.Fatal("expected iterator to be created")
	}
	if iterator.batchSize != 2 {
		t.Errorf("expected batch size 2, got %d", iterator.batchSize)
	}
	if iterator.current != 0 {
		t.Errorf("expected current position 0, got %d", iterator.current)
	}
}

func TestZeroCopyIterator_Iteration(t *testing.T) {
	// Create a sample movie list for testing
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewInMemoryFeed()
	reader := NewZeroCopyReader(blobStore, announcer)
	ctx := context.Background()
	
	setupTestData(t, blobStore, announcer)
	err := reader.RefreshTo(ctx, 1)
	if err != nil {
		t.Fatalf("failed to refresh: %v", err)
	}

	movies, err := reader.GetMovies()
	if err != nil {
		t.Fatalf("failed to get movies: %v", err)
	}

	iterator := NewZeroCopyIterator(movies, 2)

	// First batch
	if !iterator.HasNext() {
		t.Error("expected HasNext to be true initially")
	}

	batch1 := iterator.Next()
	if len(batch1) != 2 {
		t.Errorf("expected first batch size 2, got %d", len(batch1))
	}

	// Second batch
	if !iterator.HasNext() {
		t.Error("expected HasNext to be true for second batch")
	}

	batch2 := iterator.Next()
	if len(batch2) != 1 {
		t.Errorf("expected second batch size 1, got %d", len(batch2))
	}

	// No more batches
	if iterator.HasNext() {
		t.Error("expected HasNext to be false after all items consumed")
	}

	batch3 := iterator.Next()
	if batch3 != nil {
		t.Error("expected nil batch when no more items")
	}
}

func TestZeroCopyIterator_Reset(t *testing.T) {
	// Create a sample movie list for testing
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewInMemoryFeed()
	reader := NewZeroCopyReader(blobStore, announcer)
	ctx := context.Background()
	
	setupTestData(t, blobStore, announcer)
	err := reader.RefreshTo(ctx, 1)
	if err != nil {
		t.Fatalf("failed to refresh: %v", err)
	}

	movies, err := reader.GetMovies()
	if err != nil {
		t.Fatalf("failed to get movies: %v", err)
	}

	iterator := NewZeroCopyIterator(movies, 2)

	// Consume some items
	iterator.Next()
	if iterator.current != 2 {
		t.Errorf("expected current position 2, got %d", iterator.current)
	}

	// Reset
	iterator.Reset()
	if iterator.current != 0 {
		t.Errorf("expected current position 0 after reset, got %d", iterator.current)
	}

	if !iterator.HasNext() {
		t.Error("expected HasNext to be true after reset")
	}
}

func TestZeroCopyAggregator_NewZeroCopyAggregator(t *testing.T) {
	aggregator := NewZeroCopyAggregator()

	if aggregator == nil {
		t.Fatal("expected aggregator to be created")
	}
}

func TestZeroCopyAggregator_AverageRuntimeByYear(t *testing.T) {
	// Create a sample movie list for testing
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewInMemoryFeed()
	reader := NewZeroCopyReader(blobStore, announcer)
	ctx := context.Background()
	
	setupTestData(t, blobStore, announcer)
	err := reader.RefreshTo(ctx, 1)
	if err != nil {
		t.Fatalf("failed to refresh: %v", err)
	}

	movies, err := reader.GetMovies()
	if err != nil {
		t.Fatalf("failed to get movies: %v", err)
	}

	aggregator := NewZeroCopyAggregator()
	result := aggregator.AverageRuntimeByYear(movies)

	if len(result) != 3 {
		t.Errorf("expected 3 years, got %d", len(result))
	}

	// Check specific averages (sample data has specific runtimes)
	if avg, exists := result[1999]; !exists || avg != 136 {
		t.Errorf("expected average runtime 136 for 1999, got %f", avg)
	}
	if avg, exists := result[2010]; !exists || avg != 148 {
		t.Errorf("expected average runtime 148 for 2010, got %f", avg)
	}
	if avg, exists := result[2014]; !exists || avg != 169 {
		t.Errorf("expected average runtime 169 for 2014, got %f", avg)
	}
}

func TestZeroCopyAggregator_CountByGenre(t *testing.T) {
	// Create a sample movie list for testing
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewInMemoryFeed()
	reader := NewZeroCopyReader(blobStore, announcer)
	ctx := context.Background()
	
	setupTestData(t, blobStore, announcer)
	err := reader.RefreshTo(ctx, 1)
	if err != nil {
		t.Fatalf("failed to refresh: %v", err)
	}

	movies, err := reader.GetMovies()
	if err != nil {
		t.Fatalf("failed to get movies: %v", err)
	}

	aggregator := NewZeroCopyAggregator()
	result, err := aggregator.CountByGenre(movies)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check expected genre counts from sample data
	expectedCounts := map[string]int{
		"Action":   2, // Matrix and Inception
		"Sci-Fi":   2, // Matrix and Interstellar
		"Thriller": 1, // Inception
		"Drama":    1, // Interstellar
	}

	if len(result) != len(expectedCounts) {
		t.Errorf("expected %d genres, got %d", len(expectedCounts), len(result))
	}

	for genre, expectedCount := range expectedCounts {
		if count, exists := result[genre]; !exists || count != expectedCount {
			t.Errorf("expected %d movies for genre %s, got %d", expectedCount, genre, count)
		}
	}
}

func TestZeroCopyIterator_EmptyList(t *testing.T) {
	// Test with empty movie list
	emptyMovies := movie.Movie_List{}
	iterator := NewZeroCopyIterator(emptyMovies, 2)

	if iterator.HasNext() {
		t.Error("expected HasNext to be false for empty list")
	}

	batch := iterator.Next()
	if batch != nil {
		t.Error("expected nil batch for empty list")
	}
}

func TestZeroCopyAggregator_EmptyList(t *testing.T) {
	aggregator := NewZeroCopyAggregator()
	emptyMovies := movie.Movie_List{}

	// Test AverageRuntimeByYear with empty list
	result := aggregator.AverageRuntimeByYear(emptyMovies)
	if len(result) != 0 {
		t.Errorf("expected empty result for empty movie list, got %d entries", len(result))
	}

	// Test CountByGenre with empty list
	genreCounts, err := aggregator.CountByGenre(emptyMovies)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(genreCounts) != 0 {
		t.Errorf("expected empty genre counts for empty movie list, got %d entries", len(genreCounts))
	}
}

// setupTestData creates test data in the blob store for testing
func setupTestData(t *testing.T, blobStore blob.BlobStore, announcer blob.Announcer) {
	// Create a sample snapshot blob with test data
	testBlob := &blob.Blob{
		Type:     blob.SnapshotBlob,
		Version:  1,
		Data:     []byte("test data"), // Simplified for now
		Checksum: 123,
		Metadata: make(map[string]string),
	}

	err := blobStore.Store(context.Background(), testBlob)
	if err != nil {
		t.Fatalf("failed to store test blob: %v", err)
	}

	// Announce the version
	err = announcer.Announce(1)
	if err != nil {
		t.Fatalf("failed to announce version: %v", err)
	}
}
