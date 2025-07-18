package main

import (
	"fmt"
	"log"

	"github.com/leow/go-raw-hollow"
	"github.com/leow/go-raw-hollow/internal/memblob"
)

func main() {
	fmt.Println("Welcome to RAW Hollow Go!!")
	
	// Example usage
	blob := memblob.New()
	producer := hollow.NewProducer(hollow.WithBlobStager(blob))
	
	// Run a cycle
	version, err := producer.RunCycle(func(ws hollow.WriteState) error {
		return ws.Add("example data")
	})
	if err != nil {
		log.Fatal(err)
	}
	
	fmt.Printf("Created version %d\n", version)
	
	// Create consumer
	consumer := hollow.NewConsumer(
		hollow.WithBlobRetriever(blob),
		hollow.WithAnnouncementWatcher(func() (uint64, bool, error) {
			return version, true, nil
		}),
	)
	
	// Refresh consumer
	if err := consumer.Refresh(); err != nil {
		log.Fatal(err)
	}
	
	fmt.Printf("Consumer at version %d\n", consumer.CurrentVersion())
	
	// Read data
	rs := consumer.ReadState()
	if val, ok := rs.Get("example data"); ok {
		fmt.Printf("Retrieved: %v\n", val)
	}
}
