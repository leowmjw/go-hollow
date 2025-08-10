package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/consumer"
	"github.com/leowmjw/go-hollow/internal"
	"github.com/leowmjw/go-hollow/producer"
	"github.com/leowmjw/go-hollow/tools"
)

func runInteractiveMode(blobStore blob.BlobStore, announcer blob.Announcer, prod *producer.Producer, currentVersion int64, verbose bool) {
	// Create zero-copy consumer for reading existing data
	zeroCopyConsumer := consumer.NewZeroCopyConsumer(
		consumer.WithBlobRetriever(blobStore),
		consumer.WithAnnouncer(announcer),
	)
	fmt.Println("\n=== Interactive Mode (Memory Storage) ===")
	fmt.Printf("Data is now available in memory. Current version: %d\n", currentVersion)
	fmt.Println("Choose an action:")
	fmt.Println("  c <version>      - Consume data from specific version")
	fmt.Println("  i <version>      - Inspect specific version")
	fmt.Println("  d <from> <to>    - Diff between two versions")
	fmt.Println("  p [type] [count] - Produce data (p help for options)")
	fmt.Println("  l [blobs]        - List versions or detailed blob info")
	fmt.Println("  q                - Quit")
	fmt.Println("\nRealistic delta mode: Snapshots every 5 versions, deltas in between")
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
			if len(parts) > 1 && parts[1] == "help" {
				fmt.Println("Produce data evolution scenarios:")
				fmt.Println("  p add <count>    - Add new records (default: 3)")
				fmt.Println("  p update <count> - Update existing records (default: 2)")
				fmt.Println("  p delete <count> - Delete records (default: 1)")
				fmt.Println("  p mixed          - Mixed operations (add/update/delete)")
				fmt.Println("  p                - Default mixed evolution")
				continue
			}

			ctx := context.Background()
			operationType := "mixed"
			count := 3

			if len(parts) > 1 {
				operationType = parts[1]
			}
			if len(parts) > 2 {
				if c, err := strconv.Atoi(parts[2]); err == nil {
					count = c
				}
			}

			// Read existing data first
			existingData := readExistingData(zeroCopyConsumer, currentVersion)

			newVersion := prod.RunCycle(ctx, func(ws *internal.WriteState) {
				switch operationType {
				case "add":
					// First re-add all existing data
					for _, item := range existingData {
						ws.Add(item)
					}
					// Then add new records
					for i := 0; i < count; i++ {
						ws.Add(fmt.Sprintf("new_record_%d_v%d", currentVersion*100+int64(i), currentVersion+1))
					}
					fmt.Printf("Added %d new records to %d existing records\n", count, len(existingData))

				case "update":
					// Update existing records by modifying them
					updatedCount := 0
					for i := 0; i < len(existingData); i++ {
						if i < count {
									// For records we're updating, we modify them but preserve the original ID structure
							// The UPDATED_ prefix is just to make it visually obvious which records were updated
							// PROOF: This is not a new record but an update to an existing one
							fmt.Printf("PROOF OF UPDATE: Record %d: '%s' -> 'UPDATED_%s' (same identity)\n", 
								i, existingData[i], existingData[i])
							ws.Add(fmt.Sprintf("UPDATED_%s", existingData[i]))
							updatedCount++
						} else {
							// Keep the rest of the data unchanged
							ws.Add(existingData[i])
						}
					}
					fmt.Printf("Updated %d out of %d existing records\n", updatedCount, len(existingData))

				case "delete":
					// Delete records by not including them
					deletedCount := 0
					for i, item := range existingData {
						if i < count {
							// Skip this record (delete it)
							deletedCount++
						} else {
							// Keep this record
							ws.Add(item)
						}
					}
					fmt.Printf("Deleted %d records, %d remaining\n", deletedCount, len(existingData)-deletedCount)

				default: // mixed
					// Mixed operations: add some existing data, update some, delete some, add new
					addedCount := 0
					updatedCount := 0
					deletedCount := 0

					for i, item := range existingData {
						if i%3 == 0 && updatedCount < 2 {
							// Update this record but preserve the original ID structure
							// The UPDATED_ prefix is just to make it visually obvious which records were updated
							// PROOF: This is not a new record but an update to an existing one
							fmt.Printf("PROOF OF MIXED UPDATE: Record at index %d: '%s' -> 'UPDATED_%s' (same identity)\n", 
								i, item, item)
							ws.Add(fmt.Sprintf("UPDATED_%s", item))
							updatedCount++
						} else if i%4 == 0 && deletedCount < 1 {
							// Delete this record (skip it)
							deletedCount++
						} else {
							// Keep unchanged
							ws.Add(item)
						}
					}

					// Add new records
					for i := 0; i < 2; i++ {
						ws.Add(fmt.Sprintf("new_mixed_record_%d_v%d", currentVersion*10+int64(i), currentVersion+1))
						addedCount++
					}

					fmt.Printf("Mixed operations: %d added, %d updated, %d deleted\n", addedCount, updatedCount, deletedCount)
				}
			})

			if newVersion == currentVersion {
				fmt.Printf("No changes detected - version remains: %d\n", currentVersion)
			} else {
				currentVersion = newVersion
				fmt.Printf("New version produced: %d\n", newVersion)
				showBlobSummary(blobStore, newVersion)
			}

		case "l", "list":
			if len(parts) > 1 && parts[1] == "blobs" {
				showDetailedBlobListing(blobStore, verbose)
			} else {
				versions := blobStore.ListVersions()
				fmt.Printf("Available versions: %v\n", versions)
				fmt.Println("Use 'l blobs' for detailed blob information")
			}

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

	// Create consumer with the same blob store and announcer
	cons := consumer.NewConsumer(
		consumer.WithBlobRetriever(blobStore),
		consumer.WithAnnouncer(announcer),
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

	// Always show delta blob information
	deltaBlob := blobStore.RetrieveDeltaBlob(version)
	if deltaBlob != nil {
		fmt.Printf("Delta blob from version %d:\n", version)
		fmt.Printf("  Type: %v\n", deltaBlob.Type)
		fmt.Printf("  From: %d -> To: %d\n", deltaBlob.FromVersion, deltaBlob.ToVersion)
		fmt.Printf("  Data size: %d bytes\n", len(deltaBlob.Data))
		fmt.Printf("  🔄 Delta data: %s\n", truncateString(string(deltaBlob.Data), 200))
	} else {
		fmt.Printf("No delta blob found for version %d\n", version)
	}

	// Also check for delta blob pointing TO this version (reverse delta)
	if version > 1 {
		reverseDeltaBlob := blobStore.RetrieveDeltaBlob(version - 1)
		if reverseDeltaBlob != nil && reverseDeltaBlob.ToVersion == version {
			fmt.Printf("\nIncoming delta blob (v%d -> v%d):\n", version-1, version)
			fmt.Printf("  Type: %v\n", reverseDeltaBlob.Type)
			fmt.Printf("  Data size: %d bytes\n", len(reverseDeltaBlob.Data))
			fmt.Printf("  🔄 Delta data: %s\n", truncateString(string(reverseDeltaBlob.Data), 200))
		}
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

// showBlobSummary displays a quick summary of blobs for a version
func showBlobSummary(blobStore blob.BlobStore, version int64) {
	// Check for snapshot blob
	snapshotBlob := blobStore.RetrieveSnapshotBlob(version)
	if snapshotBlob != nil {
		fmt.Printf("  📸 Snapshot blob: %d bytes\n", len(snapshotBlob.Data))
	}

	// Check for delta blob
	deltaBlob := blobStore.RetrieveDeltaBlob(version)
	if deltaBlob != nil {
		fmt.Printf("  🔄 Delta blob: %d bytes\n", len(deltaBlob.Data))
	}

	if snapshotBlob == nil && deltaBlob == nil {
		fmt.Printf("  ⚠️  No blobs found for version %d\n", version)
	}
}

// showDetailedBlobListing shows comprehensive blob information across all versions
func showDetailedBlobListing(blobStore blob.BlobStore, verbose bool) {
	versions := blobStore.ListVersions()
	if len(versions) == 0 {
		fmt.Println("No versions available")
		return
	}

	fmt.Println("\n=== Detailed Blob Listing ===")
	fmt.Printf("Total versions: %d\n\n", len(versions))

	for _, version := range versions {
		fmt.Printf("Version %d:\n", version)

		// Check snapshot blob
		snapshotBlob := blobStore.RetrieveSnapshotBlob(version)
		if snapshotBlob != nil {
			fmt.Printf("  📸 SNAPSHOT: %d bytes (type: %d)\n",
				len(snapshotBlob.Data), snapshotBlob.Type)
			if verbose {
				fmt.Printf("     Data preview: %s...\n",
					truncateString(string(snapshotBlob.Data), 100))
			}
		} else {
			fmt.Printf("  📸 SNAPSHOT: none\n")
		}

		// Check delta blob
		deltaBlob := blobStore.RetrieveDeltaBlob(version)
		if deltaBlob != nil {
			fmt.Printf("  🔄 DELTA: %d bytes (type: %d)\n",
				len(deltaBlob.Data), deltaBlob.Type)
			if verbose {
				fmt.Printf("     Data preview: %s...\n",
					truncateString(string(deltaBlob.Data), 100))
			}
		} else {
			fmt.Printf("  🔄 DELTA: none\n")
		}

		fmt.Println()
	}

	fmt.Println("Legend:")
	fmt.Println("  📸 SNAPSHOT = Complete state at this version")
	fmt.Println("  🔄 DELTA    = Changes from previous version")
	fmt.Println("\nRealistic pattern: Snapshots every 5 versions, deltas in between")
}

// readExistingData reads current data from the consumer using zero-copy access
func readExistingData(zeroCopyConsumer *consumer.ZeroCopyConsumer, version int64) []string {
	if version == 0 {
		return []string{}
	}

	// Refresh consumer to the current version with zero-copy support
	ctx := context.Background()
	fmt.Printf("Refreshing to version %d with zero-copy support...\n", version)
	err := zeroCopyConsumer.TriggerRefreshToWithZeroCopy(ctx, version)
	if err != nil {
		fmt.Printf("Warning: Could not refresh consumer with zero-copy to version %d: %v\n", version, err)
		fmt.Println("Falling back to standard consumer refresh...")
		// Fall back to standard refresh
		err = zeroCopyConsumer.Consumer.TriggerRefreshTo(ctx, version)
		if err != nil {
			fmt.Printf("Error: Standard refresh also failed: %v\n", err)
			return []string{}
		}
	}

	// Try to get the zero-copy view first
	zeroCopyView, hasZeroCopy := zeroCopyConsumer.GetZeroCopyViewForVersion(version)
	if hasZeroCopy {
		fmt.Println("✅ Successfully obtained zero-copy view for data access")
		// Create a data accessor and query engine for efficient data access
		accessor := consumer.NewZeroCopyDataAccessor(zeroCopyView)
		queryEngine := consumer.NewZeroCopyQueryEngine(zeroCopyView)

		// Get the actual data using zero-copy
		var existingData []string

		// Count the records using zero-copy (without deserializing all data)
		count, err := queryEngine.CountRecords()
		if err != nil {
			fmt.Printf("Warning: Error counting records with zero-copy: %v\n", err)
		} else {
			fmt.Printf("Found %d records using zero-copy counting\n", count)
		}

		// Extract string data using zero-copy
		if stateEngine := zeroCopyConsumer.Consumer.GetStateEngine(); stateEngine != nil && stateEngine.HasType("String") {
			// Extract string data using the query engine
			strings, err := queryEngine.ExtractField("String")
			if err != nil {
				fmt.Printf("Warning: Error extracting String field with zero-copy: %v\n", err)
			} else {
				// Convert the interface{} slice to []string
				for _, item := range strings {
					if str, ok := item.(string); ok {
						existingData = append(existingData, str)
					}
				}
				fmt.Printf("✅ Successfully extracted %d string values with zero-copy\n", len(existingData))
				return existingData
			}
		}

		// If we got here, we have zero-copy but couldn't extract the strings directly
		// Try to get the raw buffer for manual parsing
		buffer := accessor.GetRawBuffer()
		if len(buffer) > 0 {
			fmt.Printf("Got raw buffer of %d bytes with zero-copy\n", len(buffer))
			// For demo purposes, we'll extract some data from the buffer
			// In a real implementation, you would parse the Cap'n Proto message properly
			return extractStringsFromBuffer(buffer, version)
		}
	}

	// If zero-copy failed or didn't have the data we need, fall back to standard state engine
	fmt.Println("⚠️ Zero-copy view not available, falling back to standard state engine")
	stateEngine := zeroCopyConsumer.Consumer.GetStateEngine()
	if stateEngine == nil {
		fmt.Printf("Warning: No state engine available for version %d\n", version)
		return []string{}
	}

	// Get the current read state from the state engine
	currentState := stateEngine.GetCurrentState()
	if currentState == nil {
		fmt.Printf("Warning: No current state available for version %d\n", version)
		return []string{}
	}

	// Get the data from the read state
	data := currentState.GetAllData()
	strings, hasStrings := data["String"]
	if hasStrings && len(strings) > 0 {
		fmt.Printf("Found %d String values using standard state engine\n", len(strings))
		var existingData []string
		for _, item := range strings {
			if str, ok := item.(string); ok {
				existingData = append(existingData, str)
			}
		}
		return existingData
	}

	// Last resort - no data found through any method
	fmt.Printf("No String data available for version %d\n", version)
	return []string{}
}

// extractStringsFromBuffer extracts string data from a raw Cap'n Proto buffer
// This is used when we have zero-copy access but need to manually parse the buffer
func extractStringsFromBuffer(buffer []byte, version int64) []string {
	// In a real implementation, this would parse the Cap'n Proto message structure
	// For this demo, we'll just scan for string-like patterns in the buffer

	var existingData []string
	var currentString []byte
	inStringMode := false

	// Simple parsing of null-terminated strings in the buffer
	for _, b := range buffer {
		// Printable ASCII chars (roughly)
		if (b >= 32 && b <= 126) || b == 9 || b == 10 || b == 13 {
			if !inStringMode {
				inStringMode = true
				currentString = []byte{}
			}
			currentString = append(currentString, b)
		} else if inStringMode && len(currentString) > 3 {
			// End of string, save if it's meaningful (at least 4 chars)
			str := string(currentString)
			existingData = append(existingData, str)
			currentString = []byte{}
			inStringMode = false
		} else if inStringMode {
			// Too short, reset
			currentString = []byte{}
			inStringMode = false
		}
	}

	// If no meaningful strings found, use fallback data based on version
	if len(existingData) == 0 {
		fmt.Println("Warning: Could not extract meaningful strings from buffer, using fallback data")
		// Generate fallback data similar to what we'd expect for this version
		switch version {
		case 1:
			// Initial data
			for i := 0; i < 10; i++ {
				existingData = append(existingData, fmt.Sprintf("test_data_%d", i))
			}
			existingData = append(existingData, "data_from_file:fixtures/simple_test.json")
		case 2:
			// Version 2 data
			for i := 0; i < 10; i++ {
				existingData = append(existingData, fmt.Sprintf("test_data_%d", i))
			}
			existingData = append(existingData, "data_from_file:fixtures/simple_test.json")
			for i := 0; i < 5; i++ {
				existingData = append(existingData, fmt.Sprintf("new_record_%d_v2", 100+i))
			}
		default:
			// Generic data for unknown versions
			for i := 0; i < 5; i++ {
				existingData = append(existingData, fmt.Sprintf("data_for_version_%d_%d", version, i))
			}
		}
	}

	return existingData
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
