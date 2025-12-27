// Zero-copy example demonstrating Cap'n Proto integration with go-hollow
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/internal"
	"github.com/leowmjw/go-hollow/producer"
	"github.com/leowmjw/go-hollow/zero_copy"
)

func main() {
	fmt.Println("=== Zero-Copy Integration Demo ===")

	ctx := context.Background()

	// Setup
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewGoroutineAnnouncer()
	defer announcer.Close()

	// Demo 1: Zero-Copy Writing
	fmt.Println("\n1. Zero-Copy Writing Demo")

	// Create producer with zero-copy serialization
	prod := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
		producer.WithSerializationMode(internal.ZeroCopyMode),
		producer.WithNumStatesBetweenSnapshots(1), // Ensure snapshot for every version
	)

	movies := []zerocopy.MovieData{
		{ID: 1, Title: "The Matrix", Year: 1999, RuntimeMin: 136, Genres: []string{"Action", "Sci-Fi"}},
		{ID: 2, Title: "Inception", Year: 2010, RuntimeMin: 148, Genres: []string{"Action", "Thriller"}},
		{ID: 3, Title: "Interstellar", Year: 2014, RuntimeMin: 169, Genres: []string{"Drama", "Sci-Fi"}},
		{ID: 4, Title: "Blade Runner 2049", Year: 2017, RuntimeMin: 164, Genres: []string{"Action", "Sci-Fi"}},
		{ID: 5, Title: "Dune", Year: 2021, RuntimeMin: 155, Genres: []string{"Action", "Adventure"}},
	}

	var err error
	var version int64
	version, err = prod.RunCycle(ctx, func(ws *internal.WriteState) {
		for _, movie := range movies {
			ws.Add(movie)
		}
	})
	if err != nil {
		log.Fatalf("RunCycle failed: %v", err)
	}
	fmt.Printf("✓ Written %d movies using zero-copy serialization (version: %d)\n", len(movies), version)

	// Demo 2: Zero-Copy Reading
	fmt.Println("\n2. Zero-Copy Reading Demo")
	reader := zerocopy.NewZeroCopyReader(blobStore, announcer)

	err = reader.RefreshTo(ctx, int64(version))
	if err != nil {
		log.Fatalf("Failed to refresh reader: %v", err)
	}
	fmt.Printf("✓ Reader refreshed to version %d\n", reader.GetCurrentVersion())

	// Demo 3: Zero-Copy Data Access
	fmt.Println("\n3. Zero-Copy Data Access Demo")
	moviesList, err := reader.GetMovies()
	if err != nil {
		log.Fatalf("Failed to get movies: %v", err)
	}

	fmt.Printf("✓ Loaded %d movies using zero-copy access\n", moviesList.Len())

	// Access individual movies without copying data
	for i := 0; i < moviesList.Len(); i++ {
		movie := moviesList.At(i)
		title, _ := movie.Title()
		genres, _ := movie.Genres()

		fmt.Printf("  Movie %d: %s (%d) - %d min",
			movie.Id(), title, movie.Year(), movie.RuntimeMin())

		if genres.Len() > 0 {
			fmt.Print(" [")
			for j := 0; j < genres.Len(); j++ {
				if j > 0 {
					fmt.Print(", ")
				}
				genre, _ := genres.At(j)
				fmt.Print(genre)
			}
			fmt.Print("]")
		}
		fmt.Println()
	}

	// Demo 4: Zero-Copy Lookup
	fmt.Println("\n4. Zero-Copy Lookup Demo")
	targetId := uint32(2)
	movie, found, err := reader.FindMovieById(targetId)
	if err != nil {
		log.Fatalf("Failed to find movie: %v", err)
	}

	if found {
		title, _ := movie.Title()
		fmt.Printf("✓ Found movie ID %d: %s (%d)\n", targetId, title, movie.Year())
	} else {
		fmt.Printf("✗ Movie ID %d not found\n", targetId)
	}

	// Demo 5: Zero-Copy Filtering
	fmt.Println("\n5. Zero-Copy Filtering Demo")
	targetYear := uint16(2010)
	moviesFromYear, err := reader.FilterMoviesByYear(targetYear)
	if err != nil {
		log.Fatalf("Failed to filter movies: %v", err)
	}

	fmt.Printf("✓ Found %d movies from year %d:\n", len(moviesFromYear), targetYear)
	for _, movie := range moviesFromYear {
		title, _ := movie.Title()
		fmt.Printf("  - %s (%d min)\n", title, movie.RuntimeMin())
	}

	// Demo 6: Zero-Copy Iteration
	fmt.Println("\n6. Zero-Copy Iteration Demo")
	iterator := zerocopy.NewZeroCopyIterator(moviesList, 2)

	batchNum := 1
	for iterator.HasNext() {
		batch := iterator.Next()
		fmt.Printf("Batch %d (%d movies):\n", batchNum, len(batch))
		for _, movie := range batch {
			title, _ := movie.Title()
			fmt.Printf("  - %s\n", title)
		}
		batchNum++
	}

	// Demo 7: Zero-Copy Aggregation
	fmt.Println("\n7. Zero-Copy Aggregation Demo")
	aggregator := zerocopy.NewZeroCopyAggregator()

	avgRuntimeByYear := aggregator.AverageRuntimeByYear(moviesList)
	fmt.Println("Average runtime by year:")
	for year, avgRuntime := range avgRuntimeByYear {
		fmt.Printf("  %d: %.1f minutes\n", year, avgRuntime)
	}

	genreCounts, err := aggregator.CountByGenre(moviesList)
	if err != nil {
		log.Fatalf("Failed to count genres: %v", err)
	}

	fmt.Println("Movie count by genre:")
	for genre, count := range genreCounts {
		fmt.Printf("  %s: %d movies\n", genre, count)
	}

	// Demo 8: Performance Comparison
	fmt.Println("\n8. Performance Comparison Demo")
	demonstratePerformanceBenefits(reader)

	fmt.Println("\n=== Zero-Copy Demo Complete ===")
}

func demonstratePerformanceBenefits(reader *zerocopy.ZeroCopyReader) {
	moviesList, err := reader.GetMovies()
	if err != nil {
		log.Printf("Failed to get movies for performance demo: %v", err)
		return
	}

	iterations := 10000

	// Zero-copy access timing
	start := time.Now()
	for i := 0; i < iterations; i++ {
		for j := 0; j < moviesList.Len(); j++ {
			movie := moviesList.At(j)
			_ = movie.Id()
			_ = movie.Year()
			_ = movie.RuntimeMin()
			title, _ := movie.Title()
			_ = title
		}
	}
	zeroCopyDuration := time.Since(start)

	// Simulated copy-based access timing (for comparison)
	type CopiedMovie struct {
		ID         uint32
		Title      string
		Year       uint16
		RuntimeMin uint16
	}

	// First, copy all data
	copiedMovies := make([]CopiedMovie, moviesList.Len())
	for i := 0; i < moviesList.Len(); i++ {
		movie := moviesList.At(i)
		title, _ := movie.Title()
		copiedMovies[i] = CopiedMovie{
			ID:         movie.Id(),
			Title:      title,
			Year:       movie.Year(),
			RuntimeMin: movie.RuntimeMin(),
		}
	}

	start = time.Now()
	for i := 0; i < iterations; i++ {
		for _, movie := range copiedMovies {
			_ = movie.ID
			_ = movie.Year
			_ = movie.RuntimeMin
			_ = movie.Title
		}
	}
	copyDuration := time.Since(start)

	fmt.Printf("Performance comparison (%d iterations):\n", iterations)
	fmt.Printf("  Zero-copy access: %v\n", zeroCopyDuration)
	fmt.Printf("  Copy-based access: %v\n", copyDuration)

	if zeroCopyDuration < copyDuration {
		speedup := float64(copyDuration) / float64(zeroCopyDuration)
		fmt.Printf("  ✓ Zero-copy is %.2fx faster\n", speedup)
	} else {
		fmt.Printf("  ⚠ Copy-based was faster (overhead from Cap'n Proto access)\n")
	}
}
