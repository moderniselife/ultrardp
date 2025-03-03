package client

import (
	"fmt"
	"log"
	"time"
	"runtime"

	"github.com/go-gl/glfw/v3.3/glfw"
)

// We know from our test that GLFW works fine - simplifying to match our successful test
// createWindows - simplified to create just one functional window first
func (c *Client) createWindows() error {
	// Just create a single window for now
	log.Println("Creating a simplified test window...")
	
	// Absolute minimal hints
	glfw.DefaultWindowHints()

	// Create a single window
	window, err := glfw.CreateWindow(400, 300, "UltraRDP Test Window", nil, nil)
	if err != nil {
		return fmt.Errorf("failed to create test window: %w", err)
	}
	
	log.Println("Test window created successfully")
	
	// Make it visible
	window.Show()
	window.SetPos(100, 100)
	
	// Process events
	glfw.PollEvents()
	
	// Store the window
	c.windows = make([]*glfw.Window, 1)
	c.windows[0] = window
	
	log.Println("Window creation completed")
	
	return nil
}

// updateDisplayLoop handles the display loop for all monitors
func (c *Client) updateDisplayLoop() {
	// GLFW event handling must run on the main thread
	fmt.Println("Locking OS thread for GLFW")
	runtime.LockOSThread()
	
	fmt.Println("Trying to initialize GLFW")
	// Initialize GLFW
	if err := glfw.Init(); err != nil {
		log.Printf("Failed to initialize GLFW: %v", err)
		return
	}
	fmt.Printf("GLFW initialized successfully, version: %s\n", glfw.GetVersionString())
	defer glfw.Terminate()
	
	// Print info about monitors
	monitors := glfw.GetMonitors()
	fmt.Printf("Found %d monitors\n", len(monitors))
	for i, monitor := range monitors {
		x, y := monitor.GetPos()
		w, h := monitor.GetVideoMode().Width, monitor.GetVideoMode().Height
		fmt.Printf("Monitor %d: %s at (%d,%d) resolution %dx%d\n", 
			i, monitor.GetName(), x, y, w, h)
	}

	// Create windows for each monitor
	if err := c.createWindows(); err != nil {  
		log.Printf("ERROR: %v", err)
		return
	}
	
	fmt.Println("Starting main render loop")
	
	// Simple window management loop
	startTime := time.Now()
	for !c.stopped && time.Since(startTime) < 60*time.Second {
		glfw.PollEvents() // Keep UI responsive
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println("Display loop terminated")
		
	// Clean up windows
	for _, w := range c.windows {
		if w != nil {
			fmt.Println("Destroying window")
			w.Destroy()
		}
	}
}