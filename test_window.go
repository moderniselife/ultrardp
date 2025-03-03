package main

import (
	"fmt"
	"log"
	"runtime"
	"time"

	"github.com/go-gl/glfw/v3.3/glfw"
)

func main() {
	fmt.Println("Starting GLFW window test")
	fmt.Println("This program will create a simple red window")

	// GLFW operations must run on the main thread
	runtime.LockOSThread()

	// Initialize GLFW
	if err := glfw.Init(); err != nil {
		log.Fatalf("Failed to initialize GLFW: %v", err)
	}
	defer glfw.Terminate()

	fmt.Println("GLFW initialized successfully")
	fmt.Printf("GLFW version: %s\n", glfw.GetVersionString())
	
	// Print info about monitors
	monitors := glfw.GetMonitors()
	fmt.Printf("Found %d monitors\n", len(monitors))
	for i, monitor := range monitors {
		x, y := monitor.GetPos()
		w, h := monitor.GetVideoMode().Width, monitor.GetVideoMode().Height
		fmt.Printf("Monitor %d: %s at (%d,%d) resolution %dx%d\n", 
			i, monitor.GetName(), x, y, w, h)
	}

	// Set window hints
	glfw.DefaultWindowHints()
	glfw.WindowHint(glfw.Visible, glfw.True)
	glfw.WindowHint(glfw.Decorated, glfw.True)
	glfw.WindowHint(glfw.Resizable, glfw.False)

	// Create window - just a simple window, no OpenGL context
	window, err := glfw.CreateWindow(400, 300, "GLFW Test Window", nil, nil)
	if err != nil {
		log.Fatalf("Failed to create window: %v", err)
	}
	defer window.Destroy()

	// Position and show window
	window.SetPos(100, 100)
	window.Show()
	fmt.Println("Window created successfully!")
	
	// Simple loop to keep window open for a few seconds
	fmt.Println("Window will close in 5 seconds")
	startTime := time.Now()
	for !window.ShouldClose() && time.Since(startTime) < 5*time.Second {
		// Poll for events
		glfw.PollEvents()
		
		// Sleep a bit to avoid hogging CPU
		time.Sleep(100 * time.Millisecond)
	}
	
	fmt.Println("Window test completed successfully")
}