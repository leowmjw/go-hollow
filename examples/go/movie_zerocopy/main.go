// Zero-copy enhanced movie catalog example demonstrating large dataset processing
// This example shows when zero-copy provides significant benefits over traditional approaches
package main

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"runtime"
	"time"

	"capnproto.org/go/capnp/v3"
	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/generated/go/movie"
	"github.com/leowmjw/go-hollow/internal"
	"github.com/leowmjw/go-hollow/producer"
	"github.com/leowmjw/go-hollow/zero_copy"
)

func main() {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	slog.Info("=== Movie Catalog Zero-Copy Demonstration ===")
	slog.Info("Scenario: Large movie database with multiple analytics consumers")

	// Setup
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewGoroutineAnnouncer()
	defer announcer.Close()

	// Demonstrate large dataset scenario where zero-copy excels
	demonstrateLargeDatasetProcessing(ctx, blobStore, announcer)

	slog.Info("=== Zero-Copy Movie Catalog Complete ===")
}

func demonstrateLargeDatasetProcessing(ctx context.Context, blobStore blob.BlobStore, announcer blob.Announcer) {
	// 1. Create a large movie dataset (simulating Netflix-scale catalog)
	slog.Info("Creating large movie dataset...", "size", "50,000 movies")
	
	start := time.Now()
	largeDataset := generateLargeMovieDataset(50000)
	generationTime := time.Since(start)
	
	slog.Info("Dataset generated", 
		"movies", len(largeDataset),
		"generation_time", generationTime,
		"avg_per_movie", generationTime/time.Duration(len(largeDataset)))

	// 2. Write using zero-copy serialization
	slog.Info("Writing dataset using zero-copy serialization...")
	
	// Create producer with zero-copy serialization
	prod := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
		producer.WithSerializationMode(internal.ZeroCopyMode),
		producer.WithNumStatesBetweenSnapshots(1), // Ensure snapshot for every version
	)
	
	start = time.Now()
	version := prod.RunCycle(ctx, func(ws *internal.WriteState) {
		for _, movieData := range largeDataset {
			ws.Add(movieData)
		}
	})
	writeTime := time.Since(start)
	
	slog.Info("Dataset written",
		"version", version,
		"write_time", writeTime,
		"movies_per_second", float64(len(largeDataset))/writeTime.Seconds())

	// 3. Create multiple consumers (simulating different analytics services)
	slog.Info("Creating multiple zero-copy consumers for different analytics...")
	


	consumers := []struct {
		name     string
		consumer *zerocopy.ZeroCopyReader
		purpose  string
	}{
		{"RecommendationEngine", zerocopy.NewZeroCopyReader(blobStore, announcer), "Generate personalized recommendations"},
		{"SearchIndexer", zerocopy.NewZeroCopyReader(blobStore, announcer), "Build search indexes"},
		{"MetricsCalculator", zerocopy.NewZeroCopyReader(blobStore, announcer), "Calculate catalog statistics"},
		{"GenreAnalyzer", zerocopy.NewZeroCopyReader(blobStore, announcer), "Analyze genre trends"},
		{"QualityScorer", zerocopy.NewZeroCopyReader(blobStore, announcer), "Score movie quality"},
	}

	// 4. All consumers read the same data simultaneously (demonstrating memory efficiency)
	slog.Info("All consumers reading the same dataset simultaneously...")
	
	var memStatsBefore runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memStatsBefore)
	
	start = time.Now()
	for i, consumer := range consumers {
		slog.Info("Consumer starting", "name", consumer.name, "purpose", consumer.purpose)
		
		// Each consumer refreshes to the same version (zero-copy sharing)
		err := consumer.consumer.RefreshTo(ctx, int64(version))
		if err != nil {
			slog.Error("Consumer refresh failed", "name", consumer.name, "error", err)
			continue
		}
		
		// Demonstrate zero-copy data access for each consumer
		demonstrateConsumerAnalytics(consumer.name, consumer.consumer, i == 0) // Detailed output for first consumer only
	}
	
	consumeTime := time.Since(start)
	
	var memStatsAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memStatsAfter)
	
	slog.Info("All consumers completed",
		"total_time", consumeTime,
		"consumers", len(consumers),
		"avg_time_per_consumer", consumeTime/time.Duration(len(consumers)))
	
	// 5. Memory efficiency analysis
	memoryUsed := memStatsAfter.Alloc - memStatsBefore.Alloc
	slog.Info("Memory efficiency analysis",
		"consumers", len(consumers),
		"total_memory_increase", fmt.Sprintf("%d KB", memoryUsed/1024),
		"memory_per_consumer", fmt.Sprintf("%d KB", memoryUsed/uint64(len(consumers))/1024),
		"note", "All consumers share the same underlying data via zero-copy")

	// 6. Performance comparison demonstration
	demonstratePerformanceComparison(largeDataset)
}

func demonstrateConsumerAnalytics(consumerName string, reader *zerocopy.ZeroCopyReader, detailed bool) {
	start := time.Now()
	
	// Get zero-copy access to movies
	movies, err := reader.GetMovies()
	if err != nil {
		slog.Error("Failed to get movies", "consumer", consumerName, "error", err)
		return
	}
	
	// Simulate different analytics workloads
	switch consumerName {
	case "RecommendationEngine":
		// Find movies by genre for recommendations
		scifiMovies, _ := reader.FilterMoviesByYear(2020) // Recent sci-fi for recommendations
		if detailed {
			slog.Info("Recommendation analysis",
				"consumer", consumerName,
				"recent_movies_found", len(scifiMovies),
				"access_method", "zero-copy filtering")
		}
		
	case "SearchIndexer":
		// Build search index (iterate all movies)
		iterator := zerocopy.NewZeroCopyIterator(movies, 1000)
		batchCount := 0
		for iterator.HasNext() {
			batch := iterator.Next()
			batchCount++
			_ = batch // Process batch for indexing
		}
		if detailed {
			slog.Info("Search indexing",
				"consumer", consumerName,
				"batches_processed", batchCount,
				"access_method", "zero-copy iteration")
		}
		
	case "MetricsCalculator":
		// Calculate statistics
		aggregator := zerocopy.NewZeroCopyAggregator()
		avgRuntime := aggregator.AverageRuntimeByYear(movies)
		genreCounts, _ := aggregator.CountByGenre(movies)
		
		if detailed {
			slog.Info("Metrics calculation",
				"consumer", consumerName,
				"years_analyzed", len(avgRuntime),
				"genres_counted", len(genreCounts),
				"access_method", "zero-copy aggregation")
		}
		
	case "GenreAnalyzer":
		// Analyze genre trends
		totalMovies := movies.Len()
		genreMap := make(map[string]int)
		
		for i := 0; i < movies.Len(); i++ {
			movie := movies.At(i)
			genres, _ := movie.Genres()
			for j := 0; j < genres.Len(); j++ {
				genre, _ := genres.At(j)
				genreMap[genre]++
			}
		}
		
		if detailed {
			slog.Info("Genre analysis",
				"consumer", consumerName,
				"total_movies", totalMovies,
				"unique_genres", len(genreMap),
				"access_method", "zero-copy field access")
		}
		
	case "QualityScorer":
		// Score movie quality based on runtime and year
		qualityScores := make(map[uint32]float64)
		
		for i := 0; i < movies.Len(); i++ {
			movie := movies.At(i)
			// Simple quality scoring algorithm
			score := float64(movie.RuntimeMin())/120.0 + float64(movie.Year()-1990)/30.0
			qualityScores[movie.Id()] = score
		}
		
		if detailed {
			slog.Info("Quality scoring",
				"consumer", consumerName,
				"movies_scored", len(qualityScores),
				"access_method", "zero-copy computation")
		}
	}
	
	processingTime := time.Since(start)
	if detailed {
		slog.Info("Consumer completed",
			"name", consumerName,
			"processing_time", processingTime,
			"movies_processed", movies.Len(),
			"movies_per_second", float64(movies.Len())/processingTime.Seconds())
	}
}

func demonstratePerformanceComparison(dataset []zerocopy.MovieData) {
	slog.Info("=== Performance Comparison: Zero-Copy vs Traditional ===")
	
	// Create Cap'n Proto dataset for zero-copy access
	_, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		slog.Error("Failed to create message", "error", err)
		return
	}
	
	movieDataset, err := movie.NewRootMovieDataset(seg)
	if err != nil {
		slog.Error("Failed to create dataset", "error", err)
		return
	}
	
	movies, err := movieDataset.NewMovies(int32(len(dataset)))
	if err != nil {
		slog.Error("Failed to create movies", "error", err)
		return
	}
	
	// Populate Cap'n Proto data
	for i, movieData := range dataset {
		m := movies.At(i)
		m.SetId(movieData.ID)
		m.SetTitle(movieData.Title)
		m.SetYear(movieData.Year)
		m.SetRuntimeMin(movieData.RuntimeMin)
		
		if len(movieData.Genres) > 0 {
			genres, _ := m.NewGenres(int32(len(movieData.Genres)))
			for j, genre := range movieData.Genres {
				genres.Set(j, genre)
			}
		}
	}
	
	iterations := 100
	
	// Zero-copy access benchmark
	start := time.Now()
	for i := 0; i < iterations; i++ {
		var totalRuntime uint64
		for j := 0; j < movies.Len(); j++ {
			movie := movies.At(j)
			title, _ := movie.Title()
			_ = title
			totalRuntime += uint64(movie.RuntimeMin())
		}
		_ = totalRuntime
	}
	zeroCopyTime := time.Since(start)
	
	// Traditional access benchmark (using copied data)
	type CopiedMovie struct {
		ID         uint32
		Title      string
		Year       uint16
		RuntimeMin uint16
		Genres     []string
	}
	
	copiedMovies := make([]CopiedMovie, len(dataset))
	for i, movieData := range dataset {
		copiedMovies[i] = CopiedMovie{
			ID:         movieData.ID,
			Title:      movieData.Title,
			Year:       movieData.Year,
			RuntimeMin: movieData.RuntimeMin,
			Genres:     movieData.Genres,
		}
	}
	
	start = time.Now()
	for i := 0; i < iterations; i++ {
		var totalRuntime uint64
		for _, movie := range copiedMovies {
			_ = movie.Title
			totalRuntime += uint64(movie.RuntimeMin)
		}
		_ = totalRuntime
	}
	traditionalTime := time.Since(start)
	
	// Report results
	slog.Info("Performance comparison results",
		"iterations", iterations,
		"movies_per_iteration", len(dataset),
		"zero_copy_time", zeroCopyTime,
		"traditional_time", traditionalTime)
	
	if traditionalTime < zeroCopyTime {
		ratio := float64(zeroCopyTime) / float64(traditionalTime)
		slog.Info("Performance analysis",
			"result", "Traditional access faster for CPU-bound operations",
			"traditional_advantage", fmt.Sprintf("%.2fx", ratio),
			"note", "Zero-copy benefits are in memory sharing and I/O scenarios")
	} else {
		ratio := float64(traditionalTime) / float64(zeroCopyTime)
		slog.Info("Performance analysis",
			"result", "Zero-copy access faster",
			"zero_copy_advantage", fmt.Sprintf("%.2fx", ratio))
	}
	
	// Memory usage comparison
	slog.Info("Memory efficiency analysis",
		"traditional_approach", "Each consumer copies entire dataset",
		"zero_copy_approach", "All consumers share single dataset",
		"memory_savings", "Linear with number of consumers",
		"best_for", "Large datasets, multiple consumers, network I/O")
}

func generateLargeMovieDataset(count int) []zerocopy.MovieData {
	movies := make([]zerocopy.MovieData, count)
	
	genres := []string{
		"Action", "Adventure", "Animation", "Biography", "Comedy", "Crime", "Documentary",
		"Drama", "Family", "Fantasy", "History", "Horror", "Music", "Mystery", "Romance",
		"Sci-Fi", "Sport", "Thriller", "War", "Western",
	}
	
	adjectives := []string{
		"Amazing", "Incredible", "Fantastic", "Epic", "Legendary", "Ultimate", "Super",
		"Great", "Awesome", "Brilliant", "Magnificent", "Spectacular", "Outstanding",
	}
	
	nouns := []string{
		"Journey", "Adventure", "Story", "Quest", "Mission", "Discovery", "Legacy",
		"Chronicles", "Saga", "Tale", "Legend", "Mystery", "Secret", "Code",
	}
	
	for i := 0; i < count; i++ {
		// Generate realistic movie data
		id := uint32(i + 1)
		
		// Generate movie title
		adjective := adjectives[rand.Intn(len(adjectives))]
		noun := nouns[rand.Intn(len(nouns))]
		title := fmt.Sprintf("The %s %s", adjective, noun)
		if i%10 == 0 {
			title = fmt.Sprintf("%s %d", title, (i/10)+1) // Add sequels
		}
		
		// Generate release year (1970-2024)
		year := uint16(1970 + rand.Intn(55))
		
		// Generate runtime (80-180 minutes)
		runtime := uint16(80 + rand.Intn(100))
		
		// Generate genres (1-3 genres per movie)
		numGenres := 1 + rand.Intn(3)
		movieGenres := make([]string, numGenres)
		usedGenres := make(map[int]bool)
		
		for j := 0; j < numGenres; j++ {
			var genreIndex int
			for {
				genreIndex = rand.Intn(len(genres))
				if !usedGenres[genreIndex] {
					usedGenres[genreIndex] = true
					break
				}
			}
			movieGenres[j] = genres[genreIndex]
		}
		
		movies[i] = zerocopy.MovieData{
			ID:         id,
			Title:      title,
			Year:       year,
			RuntimeMin: runtime,
			Genres:     movieGenres,
		}
	}
	
	return movies
}
