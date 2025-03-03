package client

import (
	"fmt"
	"log"
	"time"
	"runtime"

	"github.com/go-gl/gl/v2.1/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
)

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
	runtime.LockOSThread()
	
	// Initialize GLFW
	if err := glfw.Init(); err != nil {
		log.Printf("Failed to initialize GLFW: %v", err)
		return
	}
	log.Printf("GLFW initialized successfully, version: %s", glfw.GetVersionString())

	// Process events before window creation
	glfw.PollEvents()
	
	defer glfw.Terminate()
	
	// Create windows for each monitor
	if err := c.createWindows(); err != nil {  
		log.Printf("ERROR: %v", err)
		return
	}
	
	// Super simple - get the test window and make its context current
	if len(c.windows) == 0 || c.windows[0] == nil {
		log.Printf("No windows available for rendering")
		return
	}
	
	window := c.windows[0]
	window.MakeContextCurrent()
	
	// Initialize OpenGL
	if err := gl.Init(); err != nil {
		log.Printf("Failed to initialize OpenGL: %v", err)
		return
	}
	
	log.Println("OpenGL initialized, beginning basic render loop")
	
	// Very simple render loop - just show a blue window
	for !c.stopped && !window.ShouldClose() {
		// Poll events
		glfw.PollEvents()
		
		// Clear the window with blue to show it's working
		gl.ClearColor(0.0, 0.0, 1.0, 1.0) // Blue
		gl.Clear(gl.COLOR_BUFFER_BIT)
		
		// Swap buffers
		window.SwapBuffers()
            
		// Slight delay to prevent high CPU usage
		time.Sleep(16 * time.Millisecond) // ~60fps
	}

	log.Println("Display loop terminated")
	
	// Clean up
	for _, w := range c.windows {
		if w != nil {
			w.Destroy()
		}
	}
}