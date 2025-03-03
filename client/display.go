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
	// Set exactly the same window hints that worked in our test program
	glfw.DefaultWindowHints()
	glfw.WindowHint(glfw.Visible, glfw.True)
	glfw.WindowHint(glfw.Decorated, glfw.True)
	glfw.WindowHint(glfw.Resizable, glfw.False)
	
	// Initialize windows slice
	c.windows = make([]*glfw.Window, c.localMonitors.MonitorCount)
	
	// Create one window per monitor
	for i := uint32(0); i < c.localMonitors.MonitorCount; i++ {
		monitor := c.localMonitors.Monitors[i]
		fmt.Printf("Creating window for monitor %d\n", monitor.ID)
		
		// Create window with same pattern as test program
		window, err := glfw.CreateWindow(
			640, 480, 
			fmt.Sprintf("UltraRDP - Monitor %d", monitor.ID),
			nil, nil)
		
		if err != nil {
			fmt.Printf("ERROR: Failed to create window: %v\n", err)
			continue
		}
		
		// Position based on monitor position
		window.SetPos(int(monitor.PositionX), int(monitor.PositionY))
		window.Show()
		
		c.windows[i] = window
		fmt.Printf("Window %d created successfully\n", i+1)
		
		// Process events after each window creation
		glfw.PollEvents()
	}
	
	// Check if we created any windows
	windowCount := 0
	for _, w := range c.windows {
		if w != nil {
			windowCount++
		}
	}
	
	if windowCount == 0 {
		return fmt.Errorf("failed to create any windows")
	}
	
	fmt.Printf("Created %d windows successfully\n", windowCount)
	
	// Process events to ensure windows are visible
	glfw.PollEvents()
	
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
		fmt.Printf("Failed to initialize GLFW: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stdout, "GLFW initialized successfully, version: %s\n", glfw.GetVersionString())
	defer glfw.Terminate()
	
	// Print info about monitors
	monitors := glfw.GetMonitors()
	fmt.Fprintf(os.Stdout, "Found %d GLFW monitors\n", len(monitors))
	for i, monitor := range monitors {
		x, y := monitor.GetPos()
		w, h := monitor.GetVideoMode().Width, monitor.GetVideoMode().Height
		fmt.Fprintf(os.Stdout, "Monitor %d: %s at (%d,%d) resolution %dx%d\n", 
			i, monitor.GetName(), x, y, w, h)
	}
	
	// Create windows for each monitor
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
		time.Sleep(16 * time.Millisecond)
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