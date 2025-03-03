package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/moderniselife/ultrardp/client"
	"github.com/moderniselife/ultrardp/protocol"
)

func main() {
	// Force the display code to run on the main thread
	runtime.LockOSThread()
	
	fmt.Println("Starting simplified RDP client")
	fmt.Println("This version prioritizes window creation and display")

	// Initialize GLFW first, before anything else
	if err := glfw.Init(); err != nil {
		fmt.Printf("Failed to initialize GLFW: %v\n", err)
		os.Exit(1)
	}
	defer glfw.Terminate()
	
	fmt.Printf("GLFW initialized successfully, version: %s\n", glfw.GetVersionString())
	
	// Create a test window first to verify GLFW is working
	fmt.Println("Creating test window...")
	glfw.DefaultWindowHints()
	glfw.WindowHint(glfw.Visible, glfw.True)
	glfw.WindowHint(glfw.Decorated, glfw.True)
	glfw.WindowHint(glfw.Resizable, glfw.False)
	
	testWindow, err := glfw.CreateWindow(400, 300, "UltraRDP Test", nil, nil)
	if err != nil {
		fmt.Printf("Failed to create test window: %v\n", err)
		os.Exit(1)
	}
	
	testWindow.SetPos(100, 100)
	testWindow.Show()
	fmt.Println("Test window created successfully!")
	
	// Allow time for the window to appear
	for i := 0; i < 50; i++ {
		glfw.PollEvents()
		time.Sleep(10 * time.Millisecond)
	}
	
	// Clean up test window
	testWindow.Destroy()
	
	// Create and start client in background thread
	var clientWg sync.WaitGroup
	clientWg.Add(1)
	
	// Channel to communicate with the network thread
	stopChan := make(chan struct{})
	
	// Launch client in a separate goroutine
	go func() {
		defer clientWg.Done()
		
		// Create client
		client, err := client.NewClient("100.124.193.59:8000")
		if err != nil {
			fmt.Printf("Failed to create client: %v\n", err)
			return
		}
		
		// Start network communication
		startNetworkClient(client, stopChan)
	}()
	
	// Create actual display windows
	displayWindows := createDisplayWindows()
	
	// Handle signals for graceful shutdown
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	
	// Display loop - keep windows open until stopped
	fmt.Println("Starting display loop")
	running := true
	for running {
		// Poll for events
		glfw.PollEvents()
		
		// Check for quit
		select {
		case <-signals:
			fmt.Println("Received termination signal")
			running = false
		default:
			// Continue running
		}
		
		// Check if all windows are closed
		allClosed := true
		for _, window := range displayWindows {
			if window != nil && !window.ShouldClose() {
				allClosed = false
				break
			}
		}
		
		if allClosed {
			fmt.Println("All windows closed")
			running = false
		}
		
		// Small sleep to prevent high CPU usage
		time.Sleep(16 * time.Millisecond)
	}
	
	// Close all windows
	for _, window := range displayWindows {
		if window != nil {
			window.Destroy()
		}
	}
	
	// Stop client
	close(stopChan)
	clientWg.Wait()
	
	fmt.Println("Client terminated successfully")
}

func createDisplayWindows() []*glfw.Window {
	// Print monitor info
	monitors := glfw.GetMonitors()
	fmt.Printf("Found %d monitors\n", len(monitors))
	
	// Print detailed monitor info
	for i, monitor := range monitors {
		x, y := monitor.GetPos()
		mode := monitor.GetVideoMode()
		fmt.Printf("Monitor %d: %s at (%d,%d) resolution %dx%d\n", 
			i, monitor.GetName(), x, y, mode.Width, mode.Height)
	}

	// Create a window for each real monitor
	windows := make([]*glfw.Window, len(monitors))
	
	// Create windows one by one
	for i, monitor := range monitors {
		// Set window creation hints
		glfw.DefaultWindowHints()
		glfw.WindowHint(glfw.Visible, glfw.True)
		glfw.WindowHint(glfw.Decorated, glfw.True)
		glfw.WindowHint(glfw.Resizable, glfw.False)
		
		// Get monitor position and size
		x, y := monitor.GetPos()
		mode := monitor.GetVideoMode()
		
		// Use a fixed size for now, smaller than the monitor
		width := 800
		height := 600
		
		fmt.Printf("Creating window %d for monitor at (%d,%d)\n", i, x, y)
		
		// Create window
		window, err := glfw.CreateWindow(
			width, height,
			fmt.Sprintf("UltraRDP - Monitor %d", i),
			nil, nil)
		
		if err != nil {
			fmt.Printf("Failed to create window for monitor %d: %v\n", i, err)
			continue
		}
		
		// Position window on the monitor
		window.SetPos(x + (mode.Width - width) / 2, y + (mode.Height - height) / 2)
		window.Show()
		
		windows[i] = window
		fmt.Printf("Window %d created successfully\n", i)
		
		// Process events after each window creation
		glfw.PollEvents()
	}
	
	// Process events after all windows are created
	glfw.PollEvents()
	
	return windows
}

func startNetworkClient(c *client.Client, stopChan <-chan struct{}) {
	// Handle initial handshake
	if err := c.(*client.Client).HandleHandshake(); err != nil {
		fmt.Printf("Handshake failed: %v\n", err)
		return
	}
	
	// Simple loop to keep the client alive
	fmt.Println("Client network thread running")
	<-stopChan
	fmt.Println("Client network thread stopping")
}