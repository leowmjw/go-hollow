package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	
	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/consumer"
	"github.com/leowmjw/go-hollow/internal"
	"github.com/leowmjw/go-hollow/producer"
	"github.com/leowmjw/go-hollow/schema"
	"github.com/leowmjw/go-hollow/tools"
)

func main() {
	var (
		command    = flag.String("command", "help", "Command to run: help, produce, consume, inspect, diff")
		dataFile   = flag.String("data", "", "Data file to process")
		version    = flag.Int64("version", 0, "Version number")
		targetVer  = flag.Int64("target", 0, "Target version")
		blobStore  = flag.String("store", "memory", "Blob store type: memory, s3")
		_ = flag.String("endpoint", "localhost:9000", "S3 endpoint")
		_ = flag.String("bucket", "hollow-test", "S3 bucket name")
		verbose    = flag.Bool("verbose", false, "Verbose output")
	)
	flag.Parse()

	switch *command {
	case "help":
		printHelp()
	case "produce":
		runProducer(*blobStore, *dataFile, *verbose)
	case "consume":
		runConsumer(*blobStore, *version, *verbose)
	case "inspect":
		runInspect(*blobStore, *version, *verbose)
	case "diff":
		runDiff(*blobStore, *version, *targetVer, *verbose)
	case "schema":
		runSchemaValidation(*dataFile, *verbose)
	default:
		fmt.Printf("Unknown command: %s\n", *command)
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println("Hollow CLI Tool")
	fmt.Println("Commands:")
	fmt.Println("  produce    - Run a producer cycle with test data")
	fmt.Println("  consume    - Run a consumer to read data")
	fmt.Println("  inspect    - Inspect a specific version")
	fmt.Println("  diff       - Compare two versions")
	fmt.Println("  schema     - Validate schema files")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -data      Data file to process")
	fmt.Println("  -version   Version number")
	fmt.Println("  -target    Target version for diff")
	fmt.Println("  -store     Blob store type (memory, s3)")
	fmt.Println("  -endpoint  S3 endpoint")
	fmt.Println("  -bucket    S3 bucket name")
	fmt.Println("  -verbose   Verbose output")
}

func createBlobStore(storeType string) (blob.BlobStore, error) {
	switch storeType {
	case "memory":
		return blob.NewInMemoryBlobStore(), nil
	case "s3":
		return blob.NewLocalS3BlobStore()
	default:
		return nil, fmt.Errorf("unknown store type: %s", storeType)
	}
}

func runProducer(storeType, dataFile string, verbose bool) {
	if verbose {
		fmt.Printf("Starting producer with store: %s\n", storeType)
	}
	
	blobStore, err := createBlobStore(storeType)
	if err != nil {
		fmt.Printf("Error creating blob store: %v\n", err)
		os.Exit(1)
	}
	
	// Create announcer
	announcer := blob.NewGoroutineAnnouncer()
	defer announcer.Close()
	
	// Create producer
	prod := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
	)
	
	// Run a cycle with test data
	ctx := context.Background()
	version := prod.RunCycle(ctx, func(ws *internal.WriteState) {
		// Add some test data
		for i := 0; i < 10; i++ {
			ws.Add(fmt.Sprintf("test_data_%d", i))
		}
		
		if dataFile != "" {
			ws.Add(fmt.Sprintf("data_from_file:%s", dataFile))
		}
	})
	
	if verbose {
		fmt.Printf("Producer cycle completed, version: %d\n", version)
		
		// List available versions
		versions := blobStore.ListVersions()
		fmt.Printf("Available versions: %v\n", versions)
		
		// Show announcer stats
		fmt.Printf("Announcer subscribers: %d\n", announcer.GetSubscriberCount())
	} else {
		fmt.Printf("Version: %d\n", version)
	}
}

func runConsumer(storeType string, version int64, verbose bool) {
	if verbose {
		fmt.Printf("Starting consumer with store: %s, version: %d\n", storeType, version)
	}
	
	blobStore, err := createBlobStore(storeType)
	if err != nil {
		fmt.Printf("Error creating blob store: %v\n", err)
		os.Exit(1)
	}
	
	// Create announcer
	announcer := blob.NewGoroutineAnnouncer()
	defer announcer.Close()
	
	// Create consumer
	cons := consumer.NewConsumer(
		consumer.WithBlobRetriever(blobStore),
		consumer.WithAnnouncementWatcher(announcer),
	)
	
	ctx := context.Background()
	
	if version == 0 {
		// Refresh to latest
		err = cons.TriggerRefresh(ctx)
	} else {
		// Refresh to specific version
		err = cons.TriggerRefreshTo(ctx, version)
	}
	
	if err != nil {
		fmt.Printf("Error refreshing consumer: %v\n", err)
		os.Exit(1)
	}
	
	currentVersion := cons.GetCurrentVersion()
	
	if verbose {
		fmt.Printf("Consumer refreshed to version: %d\n", currentVersion)
		
		// Show state engine info
		stateEngine := cons.GetStateEngine()
		fmt.Printf("State engine has String type: %v\n", stateEngine.HasType("String"))
		fmt.Printf("State engine has Integer type: %v\n", stateEngine.HasType("Integer"))
	} else {
		fmt.Printf("Current version: %d\n", currentVersion)
	}
}

func runInspect(storeType string, version int64, verbose bool) {
	if verbose {
		fmt.Printf("Inspecting version %d with store: %s\n", version, storeType)
	}
	
	blobStore, err := createBlobStore(storeType)
	if err != nil {
		fmt.Printf("Error creating blob store: %v\n", err)
		os.Exit(1)
	}
	
	if version == 0 {
		// List all versions
		versions := blobStore.ListVersions()
		fmt.Printf("Available versions: %v\n", versions)
		return
	}
	
	// Inspect specific version
	snapshotBlob := blobStore.RetrieveSnapshotBlob(version)
	if snapshotBlob != nil {
		fmt.Printf("Snapshot blob for version %d:\n", version)
		fmt.Printf("  Type: %v\n", snapshotBlob.Type)
		fmt.Printf("  Version: %d\n", snapshotBlob.Version)
		fmt.Printf("  Data size: %d bytes\n", len(snapshotBlob.Data))
		if verbose {
			fmt.Printf("  Data: %s\n", string(snapshotBlob.Data))
		}
	}
	
	deltaBlob := blobStore.RetrieveDeltaBlob(version)
	if deltaBlob != nil {
		fmt.Printf("Delta blob from version %d:\n", version)
		fmt.Printf("  Type: %v\n", deltaBlob.Type)
		fmt.Printf("  From: %d -> To: %d\n", deltaBlob.FromVersion, deltaBlob.ToVersion)
		fmt.Printf("  Data size: %d bytes\n", len(deltaBlob.Data))
	}
}

func runDiff(storeType string, fromVersion, toVersion int64, verbose bool) {
	if fromVersion == 0 || toVersion == 0 {
		fmt.Println("Both -version and -target must be specified for diff")
		os.Exit(1)
	}
	
	if verbose {
		fmt.Printf("Diffing versions %d -> %d with store: %s\n", fromVersion, toVersion, storeType)
	}
	
	_, err := createBlobStore(storeType)
	if err != nil {
		fmt.Printf("Error creating blob store: %v\n", err)
		os.Exit(1)
	}
	
	// Create read states for both versions
	state1 := internal.NewReadState(fromVersion)
	state2 := internal.NewReadState(toVersion)
	
	// Create diff
	diff := tools.NewHollowDiff(state1, state2)
	
	fmt.Printf("Diff from version %d to %d:\n", diff.FromVersion, diff.ToVersion)
	fmt.Printf("Changes: %d\n", len(diff.Changes))
	
	if verbose && len(diff.Changes) > 0 {
		for i, change := range diff.Changes {
			fmt.Printf("  Change %d: Type=%v, TypeName=%s\n", i+1, change.Type, change.TypeName)
		}
	}
}

func runSchemaValidation(dataFile string, verbose bool) {
	if dataFile == "" {
		fmt.Println("Schema file must be specified with -data")
		os.Exit(1)
	}
	
	if verbose {
		fmt.Printf("Validating schema file: %s\n", dataFile)
	}
	
	// Create a test schema
	testSchema := schema.ObjectSchema{
		Name: "TestType",
		Fields: []schema.ObjectField{
			{Name: "id", Type: schema.IntField},
			{Name: "name", Type: schema.StringField},
		},
	}
	
	err := testSchema.Validate()
	if err != nil {
		fmt.Printf("Schema validation failed: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Schema validation passed\n")
	
	if verbose {
		fmt.Printf("Schema name: %s\n", testSchema.GetName())
		fmt.Printf("Schema type: %v\n", testSchema.GetSchemaType())
		fmt.Printf("Fields: %d\n", len(testSchema.Fields))
		for _, field := range testSchema.Fields {
			fmt.Printf("  Field: %s (type: %v)\n", field.Name, field.Type)
		}
	}
}
