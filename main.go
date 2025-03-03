package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	
	"github.com/moderniselife/ultrardp/client"
	"github.com/moderniselife/ultrardp/server"
)

func main() {
	// Parse command line arguments
	isServer := flag.Bool("server", false, "Run as server")
	address := flag.String("address", "localhost:8000", "Address to connect to (client) or listen on (server)")
	flag.Parse()

	// Setup logging
	log.SetOutput(os.Stdout)
	log.SetPrefix("UltraRDP: ")

	if *isServer {
		fmt.Println("Starting UltraRDP Server on", *address)
		runServer(*address)
	} else {
		fmt.Println("Starting UltraRDP Client, connecting to", *address)
		runClient(*address)
	}
}

func runServer(address string) {
	// Create and start a new server
	server, err := server.NewServer(address)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}
	
	// Start the server (this blocks until the server is stopped)
	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func runClient(address string) {
	// Create a new client
	client, err := client.NewClient(address)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	
	// Start the client (this blocks until the client is stopped)
	if err := client.Start(); err != nil {
		log.Fatalf("Client error: %v", err)
	}
}