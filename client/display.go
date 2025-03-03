package client

import (
	"fmt"
	"time"
	"os"
	"runtime"

	"github.com/go-gl/glfw/v3.3/glfw"
)

// createWindows - simplified to create just one functional window first
func (c *Client) createWindows() error {
	fmt.Fprintln(os.Stdout, "Starting createWindows function")
	
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
	
	// Initialize windows slice - use GLFW monitor count
	monitorCount := len(monitors)
	fmt.Fprintf(os.Stdout, "Creating %d windows\n", monitorCount)
	c.windows = make([]*glfw.Window, monitorCount)
	
	for i, monitor := range monitors {
		fmt.Fprintf(os.Stdout, "Creating window %d (for monitor %s)\n", i, monitor.GetName())
		
		// Window hints
		glfw.DefaultWindowHints()
		glfw.WindowHint(glfw.Visible, glfw.True)
		glfw.WindowHint(glfw.Decorated, glfw.True)
		glfw.WindowHint(glfw.Resizable, glfw.False)
		
		// Get position
		x, y := monitor.GetPos()
		mode := monitor.GetVideoMode()
		
		// Use 800x600 fixed size for debugging
		w, h := 800, 600
		
		// Title with monitor info for debugging
		title := fmt.Sprintf("UltraRDP - Monitor %d: %s", i, monitor.GetName())
		fmt.Fprintf(os.Stdout, "Window title: %s\n", title)
		
		// Create window
		fmt.Fprintf(os.Stdout, "About to create window with size %dx%d\n", w, h)
		window, err := glfw.CreateWindow(w, h, title, nil, nil)
		
		if err != nil {
			fmt.Fprintf(os.Stdout, "ERROR: Failed to create window %d: %v\n", i, err)
			continue
		}
		
		// Position centered on monitor
		centerX := x + (mode.Width - w) / 2
		centerY := y + (mode.Height - h) / 2
		fmt.Fprintf(os.Stdout, "Positioning window at (%d,%d)\n", centerX, centerY)
		window.SetPos(centerX, centerY)
		
		fmt.Fprintf(os.Stdout, "Showing window %d\n", i)
		window.Show()
		
		// Store window
		c.windows[i] = window
		fmt.Fprintf(os.Stdout, "Window %d created successfully and stored\n", i)
		
		// Process events immediately after creation
		fmt.Fprintf(os.Stdout, "Processing events after window %d creation\n", i)
		glfw.PollEvents()
		
		// CRITICAL: Add delay between window creations (was missing before)
		fmt.Fprintf(os.Stdout, "Sleeping for 100ms before next window creation\n")
		time.Sleep(100 * time.Millisecond)
	}
	
	// Additional poll events at the end
	fmt.Fprintf(os.Stdout, "Final event polling after all windows created\n")
	glfw.PollEvents()
	
	// Count how many windows were successfully created
	windowCount := 0
	for _, w := range c.windows {
		if w != nil {
			windowCount++
		}
	}
	
	fmt.Fprintf(os.Stdout, "Successfully created %d windows\n", windowCount)
	
	if windowCount == 0 {
		return fmt.Errorf("failed to create any windows")
	}
	
	return nil
}

// updateDisplayLoop handles the display loop for all monitors
func (c *Client) updateDisplayLoop() {
	fmt.Fprintln(os.Stdout, "*** Starting display loop using GLFW ***")
	
	// GLFW event handling must run on the main thread
	runtime.LockOSThread()
	
	// Initialize GLFW first
	if err := glfw.Init(); err != nil {
		fmt.Fprintf(os.Stdout, "Failed to initialize GLFW: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stdout, "GLFW initialized successfully, version: %s\n", glfw.GetVersionString())
	defer glfw.Terminate()

	// Create windows for each monitor
	fmt.Fprintln(os.Stdout, "About to create windows...")
	if err := c.createWindows(); err != nil {
		fmt.Fprintf(os.Stdout, "ERROR: %v\n", err)
		return
	}
	
	// Simple main loop - just keep windows open
	fmt.Fprintln(os.Stdout, "Entering simplified display loop with windows")
	startTime := time.Now()
	for !c.stopped && time.Since(startTime) < 60*time.Second {
		// Process events to keep windows responsive
		glfw.PollEvents()
		time.Sleep(100 * time.Millisecond)
	}
	
	fmt.Fprintln(os.Stdout, "Display loop terminated")
}