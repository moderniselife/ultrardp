package client

import (
	"fmt"
	"time"
	"os"
	"runtime"

	"github.com/go-gl/glfw/v3.3/glfw"
)

// We know from our test that GLFW works fine - simplifying to match our successful test
// createWindows - simplified to create just one functional window first
func (c *Client) createWindows() error {	
	// Get information about available monitors directly from GLFW
	monitors := glfw.GetMonitors()
	fmt.Fprintf(os.Stdout, "Found %d GLFW monitors\n", len(monitors))
	
	// Print detailed monitor info
	for i, monitor := range monitors {
		x, y := monitor.GetPos()
		mode := monitor.GetVideoMode()
		fmt.Fprintf(os.Stdout, "Monitor %d: %s at (%d,%d) resolution %dx%d\n", 
			i, monitor.GetName(), x, y, mode.Width, mode.Height)
	}
	
	// Initialize windows slice based on actual GLFW monitor count
	// This is key - use GLFW's monitor count, not our stored configuration
	c.windows = make([]*glfw.Window, len(monitors))
	
	// Create a window for each actual monitor
	for i, monitor := range monitors {
		fmt.Fprintf(os.Stdout, "Creating window %d for monitor\n", i)
		
		// Exact same window hints as our successful test
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
		
		// Create window - same exact pattern as our test program
		window, err := glfw.CreateWindow(
			width, height,
			fmt.Sprintf("UltraRDP - Monitor %d", i),
			nil, nil)
		
		if err != nil {
			fmt.Fprintf(os.Stdout, "Failed to create window for monitor %d: %v\n", i, err)
			continue
		}
		
		// Position window centrally on the monitor (like our test)
		window.SetPos(x + (mode.Width - width) / 2, y + (mode.Height - height) / 2)
		window.Show()
		
		c.windows[i] = window
		fmt.Fprintf(os.Stdout, "Window %d created successfully\n", i)
		
		// Process events after each window creation
		glfw.PollEvents()
		
		// CRITICAL: Add a short delay between window creations
		time.Sleep(100 * time.Millisecond)
	}
	
	return nil
}

// updateDisplayLoop handles the display loop for all monitors
func (c *Client) updateDisplayLoop() {
	// Write directly to stdout for debugging
	fmt.Fprintln(os.Stdout, "Starting display loop using GLFW")
	
	// GLFW event handling must run on the main thread
	runtime.LockOSThread()

	// Initialize GLFW
	if err := glfw.Init(); err != nil {
		fmt.Fprintf(os.Stdout, "Failed to initialize GLFW: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stdout, "GLFW initialized successfully, version: %s\n", glfw.GetVersionString())
	defer glfw.Terminate()
	
	// Process events before window creation
	glfw.PollEvents()
	
	// Create windows
	fmt.Fprintln(os.Stdout, "Creating windows...")
	if err := c.createWindows(); err != nil {
		fmt.Fprintf(os.Stdout, "ERROR: %v\n", err)
		return
	}
	
	// Main display loop - keep windows open until stopped
	fmt.Fprintln(os.Stdout, "Entering main display loop")
	for !c.stopped {
		// Poll for events to keep windows responsive
		glfw.PollEvents()
		
		// Check if all windows are closed
		allClosed := true
		for _, window := range c.windows {
			if window != nil && !window.ShouldClose() {
				allClosed = false
				break
			}
		}
		
		if allClosed {
			fmt.Fprintln(os.Stdout, "All windows closed, stopping client")
			c.Stop()
			break
		}
		
		// Sleep to avoid high CPU usage
		time.Sleep(50 * time.Millisecond)
	}
	
	fmt.Fprintln(os.Stdout, "Display loop terminated")
	
	// Clean up windows
	for _, w := range c.windows {
		if w != nil {
			fmt.Fprintln(os.Stdout, "Destroying window")
			w.Destroy()
		}
	}
}