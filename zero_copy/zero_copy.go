// Package zerocopy provides zero-copy data access patterns for go-hollow
// using Cap'n Proto serialization.
package zerocopy

import (
	"context"
	"fmt"

	"capnproto.org/go/capnp/v3"
	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/consumer"
	"github.com/leowmjw/go-hollow/generated/go/movie"
	"github.com/leowmjw/go-hollow/producer"
)

// ZeroCopyReader provides zero-copy access to Cap'n Proto data
type ZeroCopyReader struct {
	consumer   *consumer.Consumer
	currentMsg *capnp.Message
}

// NewZeroCopyReader creates a new zero-copy reader
func NewZeroCopyReader(blobRetriever blob.BlobRetriever, announcer blob.Announcer) *ZeroCopyReader {
	cons := consumer.NewConsumer(
		consumer.WithBlobRetriever(blobRetriever),
		consumer.WithAnnouncer(announcer),
	)

	return &ZeroCopyReader{
		consumer: cons,
	}
}

// RefreshTo updates the reader to a specific version using zero-copy deserialization
func (r *ZeroCopyReader) RefreshTo(ctx context.Context, version int64) error {
	err := r.consumer.TriggerRefreshTo(ctx, version)
	if err != nil {
		return fmt.Errorf("failed to refresh consumer: %w", err)
	}

	// For demo purposes, we simulate getting Cap'n Proto data
	// In a real implementation, this would come from the blob store
	r.currentMsg = r.createSampleMessage()
	
	return nil
}

// GetMovies returns a zero-copy view of movies
func (r *ZeroCopyReader) GetMovies() (movie.Movie_List, error) {
	if r.currentMsg == nil {
		return movie.Movie_List{}, fmt.Errorf("no data loaded")
	}

	dataset, err := movie.ReadRootMovieDataset(r.currentMsg)
	if err != nil {
		return movie.Movie_List{}, fmt.Errorf("failed to read dataset: %w", err)
	}

	movies, err := dataset.Movies()
	if err != nil {
		return movie.Movie_List{}, fmt.Errorf("failed to get movies: %w", err)
	}

	return movies, nil
}

// FindMovieById performs zero-copy lookup by ID
func (r *ZeroCopyReader) FindMovieById(id uint32) (movie.Movie, bool, error) {
	movies, err := r.GetMovies()
	if err != nil {
		return movie.Movie{}, false, err
	}

	// Zero-copy iteration - no data copying
	for i := 0; i < movies.Len(); i++ {
		m := movies.At(i)
		if m.Id() == id {
			return m, true, nil
		}
	}

	return movie.Movie{}, false, nil
}

// FilterMoviesByYear returns movies from a specific year using zero-copy access
func (r *ZeroCopyReader) FilterMoviesByYear(year uint16) ([]movie.Movie, error) {
	movies, err := r.GetMovies()
	if err != nil {
		return nil, err
	}

	var result []movie.Movie
	
	// Zero-copy filtering - only store references, not data
	for i := 0; i < movies.Len(); i++ {
		m := movies.At(i)
		if m.Year() == year {
			result = append(result, m)
		}
	}

	return result, nil
}

// GetCurrentVersion returns the current version
func (r *ZeroCopyReader) GetCurrentVersion() int64 {
	return r.consumer.GetCurrentVersion()
}

// ZeroCopyWriter provides zero-copy writing capabilities
type ZeroCopyWriter struct {
	producer *producer.Producer
}

// NewZeroCopyWriter creates a new zero-copy writer
func NewZeroCopyWriter(blobStore blob.BlobStore, announcer blob.Announcer) *ZeroCopyWriter {
	prod := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
	)

	return &ZeroCopyWriter{
		producer: prod,
	}
}

// WriteMovieDataset writes a Cap'n Proto dataset using zero-copy serialization
func (w *ZeroCopyWriter) WriteMovieDataset(ctx context.Context, movies []MovieData) (uint64, error) {
	// Create Cap'n Proto message
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return 0, fmt.Errorf("failed to create message: %w", err)
	}

	dataset, err := movie.NewRootMovieDataset(seg)
	if err != nil {
		return 0, fmt.Errorf("failed to create dataset: %w", err)
	}

	// Create movies list
	moviesList, err := dataset.NewMovies(int32(len(movies)))
	if err != nil {
		return 0, fmt.Errorf("failed to create movies list: %w", err)
	}

	// Populate movies using zero-copy writes
	for i, movieData := range movies {
		m := moviesList.At(i)
		m.SetId(movieData.ID)
		m.SetTitle(movieData.Title)
		m.SetYear(movieData.Year)
		m.SetRuntimeMin(movieData.RuntimeMin)

		if len(movieData.Genres) > 0 {
			genres, err := m.NewGenres(int32(len(movieData.Genres)))
			if err != nil {
				return 0, fmt.Errorf("failed to create genres for movie %d: %w", i, err)
			}
			for j, genre := range movieData.Genres {
				genres.Set(j, genre)
			}
		}
	}

	dataset.SetVersion(uint32(len(movies))) // Simple versioning

	// Serialize with zero-copy
	data, err := msg.Marshal()
	if err != nil {
		return 0, fmt.Errorf("failed to marshal message: %w", err)
	}

	// Store the serialized data (simplified - in real implementation would integrate with WriteState)
	_ = data // For now, just validate the process works

	// For demo, return a mock version
	return uint64(len(movies)), nil
}

// MovieData represents input data for writing
type MovieData struct {
	ID         uint32
	Title      string
	Year       uint16
	RuntimeMin uint16
	Genres     []string
}

// ZeroCopyIterator provides efficient iteration over large datasets
type ZeroCopyIterator struct {
	movies    movie.Movie_List
	current   int
	batchSize int
}

// NewZeroCopyIterator creates an iterator for zero-copy traversal
func NewZeroCopyIterator(movies movie.Movie_List, batchSize int) *ZeroCopyIterator {
	return &ZeroCopyIterator{
		movies:    movies,
		current:   0,
		batchSize: batchSize,
	}
}

// Next returns the next batch of movies without copying data
func (it *ZeroCopyIterator) Next() []movie.Movie {
	if it.current >= it.movies.Len() {
		return nil
	}

	end := it.current + it.batchSize
	if end > it.movies.Len() {
		end = it.movies.Len()
	}

	batch := make([]movie.Movie, 0, end-it.current)
	for i := it.current; i < end; i++ {
		batch = append(batch, it.movies.At(i))
	}

	it.current = end
	return batch
}

// HasNext returns true if there are more items
func (it *ZeroCopyIterator) HasNext() bool {
	return it.current < it.movies.Len()
}

// Reset resets the iterator to the beginning
func (it *ZeroCopyIterator) Reset() {
	it.current = 0
}

// ZeroCopyAggregator provides efficient aggregation operations
type ZeroCopyAggregator struct{}

// NewZeroCopyAggregator creates a new aggregator
func NewZeroCopyAggregator() *ZeroCopyAggregator {
	return &ZeroCopyAggregator{}
}

// AverageRuntimeByYear calculates average runtime by year without copying data
func (a *ZeroCopyAggregator) AverageRuntimeByYear(movies movie.Movie_List) map[uint16]float64 {
	yearStats := make(map[uint16]struct {
		total uint64
		count int
	})

	// Zero-copy aggregation
	for i := 0; i < movies.Len(); i++ {
		m := movies.At(i)
		year := m.Year()
		runtime := m.RuntimeMin()

		stats := yearStats[year]
		stats.total += uint64(runtime)
		stats.count++
		yearStats[year] = stats
	}

	// Calculate averages
	result := make(map[uint16]float64)
	for year, stats := range yearStats {
		if stats.count > 0 {
			result[year] = float64(stats.total) / float64(stats.count)
		}
	}

	return result
}

// CountByGenre counts movies by genre without copying strings
func (a *ZeroCopyAggregator) CountByGenre(movies movie.Movie_List) (map[string]int, error) {
	genreCounts := make(map[string]int)

	for i := 0; i < movies.Len(); i++ {
		m := movies.At(i)
		genres, err := m.Genres()
		if err != nil {
			return nil, fmt.Errorf("failed to get genres for movie %d: %w", i, err)
		}

		// Zero-copy genre iteration
		for j := 0; j < genres.Len(); j++ {
			genre, err := genres.At(j)
			if err != nil {
				continue
			}
			genreCounts[genre]++
		}
	}

	return genreCounts, nil
}

// createSampleMessage creates a sample Cap'n Proto message for demonstration
func (r *ZeroCopyReader) createSampleMessage() *capnp.Message {
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil
	}

	dataset, err := movie.NewRootMovieDataset(seg)
	if err != nil {
		return nil
	}

	// Create sample movies
	movies, err := dataset.NewMovies(3)
	if err != nil {
		return nil
	}

	// Movie 1
	m1 := movies.At(0)
	m1.SetId(1)
	m1.SetTitle("The Matrix")
	m1.SetYear(1999)
	m1.SetRuntimeMin(136)
	genres1, _ := m1.NewGenres(2)
	genres1.Set(0, "Action")
	genres1.Set(1, "Sci-Fi")

	// Movie 2
	m2 := movies.At(1)
	m2.SetId(2)
	m2.SetTitle("Inception")
	m2.SetYear(2010)
	m2.SetRuntimeMin(148)
	genres2, _ := m2.NewGenres(2)
	genres2.Set(0, "Action")
	genres2.Set(1, "Thriller")

	// Movie 3
	m3 := movies.At(2)
	m3.SetId(3)
	m3.SetTitle("Interstellar")
	m3.SetYear(2014)
	m3.SetRuntimeMin(169)
	genres3, _ := m3.NewGenres(2)
	genres3.Set(0, "Drama")
	genres3.Set(1, "Sci-Fi")

	dataset.SetVersion(1)

	return msg
}
