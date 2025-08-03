package main

import (
	"context"
	"fmt"
	"testing"
	"time"
	"unsafe"

	"capnproto.org/go/capnp/v3"
	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/consumer"
	"github.com/leowmjw/go-hollow/generated/go/movie"
	"github.com/leowmjw/go-hollow/internal"
	"github.com/leowmjw/go-hollow/producer"
)

// ZeroCopyMovieData represents data using Cap'n Proto structs for zero-copy access
type ZeroCopyMovieData struct {
	Movie   movie.Movie
	Message *capnp.Message
}

// BenchmarkZeroCopyVsCopy compares zero-copy vs copy-based data access
func BenchmarkZeroCopyVsCopy(b *testing.B) {
	sizes := []struct {
		name string
		size int
	}{
		{"1K", 1000},
		{"10K", 10000},
		{"100K", 100000},
	}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("ZeroCopy_%s", size.name), func(b *testing.B) {
			benchmarkZeroCopyAccess(b, size.size)
		})
		b.Run(fmt.Sprintf("Copy_%s", size.name), func(b *testing.B) {
			benchmarkCopyAccess(b, size.size)
		})
	}
}

func benchmarkZeroCopyAccess(b *testing.B, recordCount int) {
	// Setup: Create Cap'n Proto data once
	zeroCopyData := generateZeroCopyMovieData(recordCount)
	
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Zero-copy access: directly read from Cap'n Proto structs
		var totalRuntime uint64
		for _, movieData := range zeroCopyData {
			// Direct field access without copying data
			title, _ := movieData.Movie.Title()
			_ = title // Use the title
			year := movieData.Movie.Year()
			runtime := movieData.Movie.RuntimeMin()
			genres, _ := movieData.Movie.Genres()
			genreCount := genres.Len()
			
			totalRuntime += uint64(runtime)
			_ = year
			_ = genreCount
		}
		
		// Prevent optimization
		if totalRuntime == 0 {
			b.Fatal("Expected non-zero runtime")
		}
	}

	// Report metrics
	recordsPerSec := float64(recordCount*b.N) / b.Elapsed().Seconds()
	b.ReportMetric(recordsPerSec, "records/sec")
	b.ReportMetric(float64(recordCount*b.N)/1e6, "million_records_processed")
}

func benchmarkCopyAccess(b *testing.B, recordCount int) {
	// Setup: Create traditional Go structs (requires copying)
	copyData := generateMovieData(recordCount)
	
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Copy-based access: data is copied into Go structs
		var totalRuntime uint64
		for _, movieData := range copyData {
			// Access copied data
			title := movieData.Title
			_ = title
			year := movieData.Year
			runtime := movieData.RuntimeMin
			genreCount := len(movieData.Genres)
			
			totalRuntime += uint64(runtime)
			_ = year
			_ = genreCount
		}
		
		// Prevent optimization
		if totalRuntime == 0 {
			b.Fatal("Expected non-zero runtime")
		}
	}

	// Report metrics
	recordsPerSec := float64(recordCount*b.N) / b.Elapsed().Seconds()
	b.ReportMetric(recordsPerSec, "records/sec")
	b.ReportMetric(float64(recordCount*b.N)/1e6, "million_records_processed")
}

// BenchmarkZeroCopyProducerConsumer tests end-to-end zero-copy performance
func BenchmarkZeroCopyProducerConsumer(b *testing.B) {
	ctx := context.Background()
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewGoroutineAnnouncer()
	defer announcer.Close()

	prod := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
	)

	cons := consumer.NewConsumer(
		consumer.WithBlobRetriever(blobStore),
		consumer.WithAnnouncementWatcher(announcer),
	)

	// Generate Cap'n Proto data for production
	recordCount := 10000
	zeroCopyData := generateZeroCopyMovieData(recordCount)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Producer writes zero-copy data
		version, err := prod.RunCycleE(ctx, func(ws *internal.WriteState) {
			for _, movieData := range zeroCopyData {
				// Convert Cap'n Proto struct to internal format
				title, _ := movieData.Movie.Title()
				ws.Add(CapnProtoMovieData{
					ID:         movieData.Movie.Id(),
					Title:      title,
					Year:       movieData.Movie.Year(),
					RuntimeMin: movieData.Movie.RuntimeMin(),
				})
			}
		})

		if err != nil {
			b.Fatalf("Producer cycle failed: %v", err)
		}

		// Consumer reads the data
		err = cons.TriggerRefreshTo(ctx, int64(version))
		if err != nil {
			b.Fatalf("Consumer refresh failed: %v", err)
		}

		currentVersion := cons.GetCurrentVersion()
		if currentVersion != int64(version) {
			b.Fatalf("Version mismatch: expected %d, got %d", version, currentVersion)
		}
	}

	// Report throughput metrics
	totalRecords := float64(recordCount * b.N)
	recordsPerSec := totalRecords / b.Elapsed().Seconds()
	b.ReportMetric(recordsPerSec, "records/sec")
	b.ReportMetric(totalRecords/1e6, "million_records")
}

// BenchmarkMemoryFootprintZeroCopy measures memory usage with zero-copy access
func BenchmarkMemoryFootprintZeroCopy(b *testing.B) {
	recordCount := 1000
	
	b.Run("ZeroCopy", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			data := generateZeroCopyMovieData(recordCount)
			// Access data without copying
			var totalSize uint64
			for _, movieData := range data {
				title, _ := movieData.Movie.Title()
				totalSize += uint64(len(title))
				totalSize += uint64(unsafe.Sizeof(movieData.Movie.Year()))
			}
			_ = totalSize
		}
	})

	b.Run("Copy", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			data := generateMovieData(recordCount)
			// Access copied data
			var totalSize uint64
			for _, movieData := range data {
				totalSize += uint64(len(movieData.Title))
				totalSize += uint64(unsafe.Sizeof(movieData.Year))
			}
			_ = totalSize
		}
	})
}

// BenchmarkSerializationZeroCopy tests serialization performance
func BenchmarkSerializationZeroCopy(b *testing.B) {
	recordCount := 1000

	b.Run("CapnProto_Serialize", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// Create a Cap'n Proto message
			msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
			if err != nil {
				b.Fatal(err)
			}

			dataset, err := movie.NewRootMovieDataset(seg)
			if err != nil {
				b.Fatal(err)
			}

			movies, err := dataset.NewMovies(int32(recordCount))
			if err != nil {
				b.Fatal(err)
			}

			// Fill with data
			for j := 0; j < recordCount; j++ {
				m := movies.At(j)
				m.SetId(uint32(j + 1))
				m.SetTitle(fmt.Sprintf("Movie %d", j+1))
				m.SetYear(uint16(2000 + (j % 25)))
				m.SetRuntimeMin(uint16(90 + (j % 90)))
			}

			// Serialize to bytes
			data, err := msg.Marshal()
			if err != nil {
				b.Fatal(err)
			}
			_ = data
		}
	})

	b.Run("Traditional_Serialize", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// Create traditional structs and serialize (simulated)
			movies := make([]TestMovieData, recordCount)
			for j := 0; j < recordCount; j++ {
				movies[j] = TestMovieData{
					ID:         uint32(j + 1),
					Title:      fmt.Sprintf("Movie %d", j+1),
					Year:       uint16(2000 + (j % 25)),
					RuntimeMin: uint16(90 + (j % 90)),
					Genres:     []string{"Action", "Drama"},
				}
			}
			
			// Simulate serialization cost
			var totalBytes int
			for _, movie := range movies {
				totalBytes += len(movie.Title)
				totalBytes += len(movie.Genres) * 10 // estimate
				totalBytes += 8 // other fields
			}
			_ = totalBytes
		}
	})
}

// BenchmarkDeserializationZeroCopy tests deserialization performance
func BenchmarkDeserializationZeroCopy(b *testing.B) {
	// Pre-create serialized Cap'n Proto data
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		b.Fatal(err)
	}

	dataset, err := movie.NewRootMovieDataset(seg)
	if err != nil {
		b.Fatal(err)
	}

	recordCount := 1000
	movies, err := dataset.NewMovies(int32(recordCount))
	if err != nil {
		b.Fatal(err)
	}

	for j := 0; j < recordCount; j++ {
		m := movies.At(j)
		m.SetId(uint32(j + 1))
		m.SetTitle(fmt.Sprintf("Movie %d", j+1))
		m.SetYear(uint16(2000 + (j % 25)))
		m.SetRuntimeMin(uint16(90 + (j % 90)))
	}

	serializedData, err := msg.Marshal()
	if err != nil {
		b.Fatal(err)
	}

	b.Run("CapnProto_Deserialize", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// Zero-copy deserialization
			msg, err := capnp.Unmarshal(serializedData)
			if err != nil {
				b.Fatal(err)
			}

			dataset, err := movie.ReadRootMovieDataset(msg)
			if err != nil {
				b.Fatal(err)
			}

			movies, err := dataset.Movies()
			if err != nil {
				b.Fatal(err)
			}

			// Access first movie to ensure deserialization
			if movies.Len() > 0 {
				firstMovie := movies.At(0)
				title, _ := firstMovie.Title()
				year := firstMovie.Year()
				_ = title
				_ = year
			}
		}
	})
}

// generateZeroCopyMovieData creates Cap'n Proto movie data for zero-copy benchmarks
func generateZeroCopyMovieData(count int) []ZeroCopyMovieData {
	result := make([]ZeroCopyMovieData, count)
	genres := []string{"Action", "Drama", "Comedy", "Thriller", "Sci-Fi", "Romance"}

	for i := 0; i < count; i++ {
		// Create a new Cap'n Proto message for each movie
		msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
		if err != nil {
			panic(err)
		}

		movie, err := movie.NewRootMovie(seg)
		if err != nil {
			panic(err)
		}

		// Set movie data
		movie.SetId(uint32(i + 1))
		movie.SetTitle(fmt.Sprintf("Test Movie %d", i+1))
		movie.SetYear(uint16(1990 + (i % 35)))
		movie.SetRuntimeMin(uint16(90 + (i % 90)))

		// Set genres
		genreList, err := movie.NewGenres(2)
		if err != nil {
			panic(err)
		}
		genreList.Set(0, genres[i%len(genres)])
		genreList.Set(1, genres[(i+1)%len(genres)])

		result[i] = ZeroCopyMovieData{
			Movie:   movie,
			Message: msg,
		}
	}

	return result
}

// CapnProtoMovieData is a simplified struct for producer integration
type CapnProtoMovieData struct {
	ID         uint32
	Title      string
	Year       uint16
	RuntimeMin uint16
}

// BenchmarkCapnProtoStructCreation measures the cost of creating Cap'n Proto structs
func BenchmarkCapnProtoStructCreation(b *testing.B) {
	b.Run("CapnProto_Creation", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
			if err != nil {
				b.Fatal(err)
			}

			movie, err := movie.NewRootMovie(seg)
			if err != nil {
				b.Fatal(err)
			}

			movie.SetId(uint32(i))
			movie.SetTitle(fmt.Sprintf("Movie %d", i))
			movie.SetYear(2024)
			movie.SetRuntimeMin(120)

			_ = msg // Keep message alive
		}
	})

	b.Run("Go_Struct_Creation", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			movie := TestMovieData{
				ID:         uint32(i),
				Title:      fmt.Sprintf("Movie %d", i),
				Year:       2024,
				RuntimeMin: 120,
				Genres:     []string{"Action"},
			}
			_ = movie
		}
	})
}

// BenchmarkRatingWithTimestamp benchmarks complex nested structures
func BenchmarkRatingWithTimestamp(b *testing.B) {
	b.Run("CapnProto_Rating", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
			if err != nil {
				b.Fatal(err)
			}

			rating, err := movie.NewRootRating(seg)
			if err != nil {
				b.Fatal(err)
			}

			rating.SetMovieId(uint32(i))
			rating.SetUserId(uint32(i + 1000))
			rating.SetScore(4.5)

			// Create nested timestamp
			timestamp, err := rating.NewTimestamp()
			if err != nil {
				b.Fatal(err)
			}
			timestamp.SetUnixSeconds(uint64(time.Now().Unix()))
			timestamp.SetNanos(uint32(time.Now().Nanosecond()))

			_ = msg
		}
	})

	b.Run("Go_Rating", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rating := struct {
				MovieId   uint32
				UserId    uint32
				Score     float32
				Timestamp time.Time
			}{
				MovieId:   uint32(i),
				UserId:    uint32(i + 1000),
				Score:     4.5,
				Timestamp: time.Now(),
			}
			_ = rating
		}
	})
}
