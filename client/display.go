package client

import (
	"fmt"
	"log"
	"runtime"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
)

// updateDisplayLoop handles the display loop for all monitors
func (c *Client) updateDisplayLoop() {
	// GLFW event handling must run on the main thread
	runtime.LockOSThread()

	// Initialize GLFW
	if err := glfw.Init(); err != nil {
		log.Printf("Failed to initialize GLFW: %v", err)
		return
	}
	defer glfw.Terminate()

	// Create windows for each mapped monitor
	if err := c.createWindows(); err != nil {
		log.Printf("Failed to create windows: %v", err)
		return
	}

	// Initialize OpenGL for each window
	for _, window := range c.windows {
		window.MakeContextCurrent()
		if err := gl.Init(); err != nil {
			log.Printf("Failed to initialize OpenGL: %v", err)
			return
		}
	}

	// Main display loop
	for !c.stopped {
		c.frameMutex.Lock()
		for i, window := range c.windows {
			if window.ShouldClose() {
				c.Stop()
				break
			}

			// Make this window's context current
			window.MakeContextCurrent()

			// Get the frame buffer for this monitor
			monitorID := c.localMonitors.Monitors[i].ID
			frameData := c.frameBuffers[monitorID]

			// Render the frame
			c.renderFrame(window, frameData)
		}
		c.frameMutex.Unlock()

		// Process events
		glfw.PollEvents()
	}

	// Cleanup
	for _, window := range c.windows {
		window.Destroy()
	}
}

// createWindows creates a window for each mapped monitor
func (c *Client) createWindows() error {
	c.windows = make([]*glfw.Window, 0, c.localMonitors.MonitorCount)

	// Set window hints
	glfw.WindowHint(glfw.ContextVersionMajor, 4)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)

	// Create a window for each monitor
	for _, monitor := range c.localMonitors.Monitors {
		// Create window
		window, err := glfw.CreateWindow(
			int(monitor.Width),
			int(monitor.Height),
			fmt.Sprintf("UltraRDP - Monitor %d", monitor.ID),
			nil,
			nil,
		)
		if err != nil {
			return fmt.Errorf("failed to create window for monitor %d: %w", monitor.ID, err)
		}

		// Set window position
		window.SetPos(int(monitor.PositionX), int(monitor.PositionY))

		// Add to windows slice
		c.windows = append(c.windows, window)
	}

	return nil
}

// renderFrame renders a frame to the specified window
func (c *Client) renderFrame(window *glfw.Window, frameData []byte) {
	// TODO: Implement frame rendering using OpenGL
	// This would involve:
	// 1. Creating and updating textures from decoded frame data
	// 2. Using shaders to render the textures
	// 3. Handling different pixel formats and color spaces
	// 4. Implementing proper scaling and aspect ratio handling

	// For now, just clear the window to indicate it's working
	gl.ClearColor(0.2, 0.3, 0.3, 1.0)
	gl.Clear(gl.COLOR_BUFFER_BIT)

	// Swap buffers
	window.SwapBuffers()
}