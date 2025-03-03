package client

import (
	"log"
	"runtime"
	"testing"
	"time"

	"github.com/go-gl/glfw/v3.3/glfw"
)

// TestGLFWWindow is a simple test to verify GLFW window creation works
func TestGLFWWindow(t *testing.T) {
	// GLFW operations must run on the main thread
	runtime.LockOSThread()

	if err := glfw.Init(); err != nil {
		t.Fatalf("Failed to initialize GLFW: %v", err)
	}
	defer glfw.Terminate()

	log.Println("GLFW initialized successfully, version:", glfw.GetVersionString())

	// Very simple window hints
	glfw.DefaultWindowHints()
	glfw.WindowHint(glfw.Visible, glfw.True)
	glfw.WindowHint(glfw.Decorated, glfw.True)
	glfw.WindowHint(glfw.Resizable, glfw.False)

	// Create window
	window, err := glfw.CreateWindow(400, 300, "GLFW Test Window", nil, nil)
	if err != nil {
		t.Fatalf("Failed to create window: %v", err)
	}
	defer window.Destroy()

	window.SetPos(100, 100)
	window.Show()
	
	// Make window visible for a few seconds
	log.Println("Window created successfully! Window should be visible now.")
	for i := 0; i < 10; i++ {
		window.MakeContextCurrent()
		
		// Fill window with a bright color so it's easy to see
		// This doesn't use OpenGL to avoid any potential issues
		
		// Process events
		glfw.PollEvents()
		window.SwapBuffers()
		
		log.Printf("Window update loop iteration %d", i)
		time.Sleep(500 * time.Millisecond)
	}
	
	log.Println("Test completed successfully")
}