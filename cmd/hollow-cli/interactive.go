package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/consumer"
	"github.com/leowmjw/go-hollow/internal"
	"github.com/leowmjw/go-hollow/producer"
	"github.com/leowmjw/go-hollow/tools"
)

func runInteractiveMode(blobStore blob.BlobStore, announcer blob.Announcer, prod *producer.Producer, currentVersion int64, verbose bool) {
	fmt.Println("\n=== Interactive Mode (Memory Storage) ===")
	fmt.Printf("Data is now available in memory. Current version: %d\n", currentVersion)
	fmt.Println("Choose an action:")
	fmt.Println("  c <version> - Consume data from specific version")
	fmt.Println("  i <version> - Inspect specific version")
	fmt.Println("  d <from> <to> - Diff between two versions")
	fmt.Println("  p - Produce another version")
	fmt.Println("  l - List available versions")
	fmt.Println("  q - Quit")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("hollow> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		parts := strings.Fields(input)
		if len(parts) == 0 {
			continue
		}

		command := strings.ToLower(parts[0])
		switch command {
		case "c", "consume":
			if len(parts) < 2 {
				fmt.Println("Usage: c <version>")
				continue
			}
			version := parseVersion(parts[1])
			if version == -1 {
				fmt.Println("Invalid version number")
				continue
			}
			runConsumerInMemory(blobStore, announcer, version, verbose)

		case "i", "inspect":
			if len(parts) < 2 {
				fmt.Println("Usage: i <version>")
				continue
			}
			version := parseVersion(parts[1])
			if version == -1 {
				fmt.Println("Invalid version number")
				continue
			}
			runInspectInMemory(blobStore, version, verbose)

		case "d", "diff":
			if len(parts) < 3 {
				fmt.Println("Usage: d <from_version> <to_version>")
				continue
			}
			fromVersion := parseVersion(parts[1])
			toVersion := parseVersion(parts[2])
			if fromVersion == -1 || toVersion == -1 {
				fmt.Println("Invalid version numbers")
				continue
			}
			runDiffInMemory(blobStore, fromVersion, toVersion, verbose)

		case "p", "produce":
			ctx := context.Background()
			newVersion := prod.RunCycle(ctx, func(ws *internal.WriteState) {
				// Add some new test data
				for i := 0; i < 5; i++ {
					ws.Add(fmt.Sprintf("new_data_v%d_%d", currentVersion+1, i))
				}
			})
			currentVersion = newVersion
			fmt.Printf("New version produced: %d\n", newVersion)

		case "l", "list":
			versions := blobStore.ListVersions()
			fmt.Printf("Available versions: %v\n", versions)

		case "q", "quit", "exit":
			fmt.Println("Goodbye!")
			return

		default:
			fmt.Printf("Unknown command: %s\n", command)
			fmt.Println("Available commands: c, i, d, p, l, q")
		}
		fmt.Println()
	}
}

func parseVersion(s string) int64 {
	var version int64
	_, err := fmt.Sscanf(s, "%d", &version)
	if err != nil {
		return -1
	}
	return version
}

func runConsumerInMemory(blobStore blob.BlobStore, announcer blob.Announcer, version int64, verbose bool) {
	if verbose {
		fmt.Printf("Starting in-memory consumer for version: %d\n", version)
	}

	// Convert announcer to AnnouncementWatcher interface
	watcher, ok := announcer.(blob.AnnouncementWatcher)
	if !ok {
		fmt.Printf("Error: Announcer does not implement AnnouncementWatcher interface\n")
		return
	}

	// Create consumer with the same blob store and announcer
	cons := consumer.NewConsumer(
		consumer.WithBlobRetriever(blobStore),
		consumer.WithAnnouncementWatcher(watcher),
	)

	ctx := context.Background()

	var err error
	if version == 0 {
		// Refresh to latest
		err = cons.TriggerRefresh(ctx)
	} else {
		// Refresh to specific version
		err = cons.TriggerRefreshTo(ctx, version)
	}

	if err != nil {
		fmt.Printf("Error refreshing consumer: %v\n", err)
		return
	}

	currentVersion := cons.GetCurrentVersion()
	fmt.Printf("Consumer successfully refreshed to version: %d\n", currentVersion)

	// Show basic state engine info
	stateEngine := cons.GetStateEngine()
	fmt.Printf("State engine ready with version: %d\n", stateEngine.GetCurrentVersion())

	if verbose {
		fmt.Printf("State engine has String type: %v\n", stateEngine.HasType("String"))
		fmt.Printf("State engine has Integer type: %v\n", stateEngine.HasType("Integer"))
		// TODO: Add more detailed data inspection when API is available
	}
}

func runInspectInMemory(blobStore blob.BlobStore, version int64, verbose bool) {
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
	} else {
		fmt.Printf("No snapshot blob found for version %d\n", version)
	}

	deltaBlob := blobStore.RetrieveDeltaBlob(version)
	if deltaBlob != nil {
		fmt.Printf("Delta blob from version %d:\n", version)
		fmt.Printf("  Type: %v\n", deltaBlob.Type)
		fmt.Printf("  From: %d -> To: %d\n", deltaBlob.FromVersion, deltaBlob.ToVersion)
		fmt.Printf("  Data size: %d bytes\n", len(deltaBlob.Data))
	} else {
		fmt.Printf("No delta blob found for version %d\n", version)
	}
}

func runDiffInMemory(blobStore blob.BlobStore, fromVersion, toVersion int64, verbose bool) {
	fmt.Printf("Diffing versions %d -> %d with in-memory store\n", fromVersion, toVersion)

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
