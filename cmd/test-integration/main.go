package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"photonic/internal/darktable"
	"photonic/internal/storage"
	"photonic/internal/tasks"
)

func main() {
	fmt.Println("🔍 Testing Darktable + Filesystem Integration")

	// Setup storage
	store, err := storage.New("test_integration.db")
	if err != nil {
		log.Fatal("Failed to create storage:", err)
	}
	defer store.Close()

	// Test darktable database connection
	dtDB, err := darktable.NewDarktableDB("")
	if err != nil {
		log.Fatal("Failed to create darktable connection:", err)
	}

	if err := dtDB.Connect(); err != nil {
		log.Fatal("Failed to connect to darktable:", err)
	}
	defer dtDB.Close()

	fmt.Println("✅ Connected to darktable database")

	// Get basic stats
	stats, err := dtDB.GetStats()
	if err != nil {
		log.Fatal("Failed to get stats:", err)
	}

	fmt.Printf("📊 Darktable Library Stats:\n")
	fmt.Printf("   Total Images: %d\n", stats["total_images"])
	fmt.Printf("   Edited Images: %d\n", stats["edited_images"])
	fmt.Printf("   Film Rolls: %d\n", stats["film_rolls"])
	fmt.Printf("   Edit Percentage: %.1f%%\n", stats["edit_percentage"])

	if earliest, ok := stats["earliest_photo"].(time.Time); ok {
		fmt.Printf("   Earliest Photo: %s\n", earliest.Format("2006-01-02"))
	}
	if latest, ok := stats["latest_photo"].(time.Time); ok {
		fmt.Printf("   Latest Photo: %s\n", latest.Format("2006-01-02"))
	}

	// Test dual watcher setup
	fmt.Println("\n🚀 Setting up dual watcher...")

	watchPaths := []string{"/data/Photography"}
	dualWatcher, err := tasks.NewDualWatcher(watchPaths, store, "")
	if err != nil {
		log.Fatal("Failed to create dual watcher:", err)
	}

	fmt.Println("✅ Dual watcher created successfully")

	// Start monitoring
	if err := dualWatcher.Start(); err != nil {
		log.Fatal("Failed to start dual watcher:", err)
	}
	defer dualWatcher.Stop()

	fmt.Println("🎯 Starting 30-second monitoring test...")

	// Monitor for 30 seconds
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	events := dualWatcher.GetEnrichedEvents()
	eventCount := 0

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("\n✅ Test completed. Captured %d events.\n", eventCount)
			return
		case event := <-events:
			eventCount++
			fmt.Printf("📸 Event: %s - %s", event.EventType, event.FilePath)
			if event.IsInDarktable {
				fmt.Printf(" [In Darktable]")
			}
			if event.IsProcessed {
				fmt.Printf(" [Processed]")
			}
			if event.IsNewImport {
				fmt.Printf(" [New Import]")
			}
			fmt.Println()
		case <-time.After(10 * time.Second):
			fmt.Println("⏳ No events in last 10 seconds...")
		}
	}
}
