package main

import (
	"log"
	"time"

	"nodestorage/v2/example/guild_territory"
)

func main() {
	// Set up logging
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Println("Starting Guild Territory Construction Example")

	// Run the example
	guild_territory.Example()

	// Wait a bit to ensure all logs are printed
	time.Sleep(time.Second)
	log.Println("Example completed")
}
