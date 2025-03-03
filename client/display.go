package client

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"log"
	"runtime"

	"github.com/go-gl/gl/v2.1/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
)

// createWindows creates GLFW windows for each mapped monitor with improved reliability
func (c *Client) createWindows() error {
	// Initialize windows slice
	c.windows = make([]*glfw.Window, c.localMonitors.MonitorCount)
	
	// Set window creation hints for maximum compatibility
	glfw.DefaultWindowHints()
	glfw.WindowHint(glfw.ContextVersionMajor, 2)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)
	glfw.WindowHint(glfw.Resizable, glfw.False)
	glfw.WindowHint(glfw.Visible, glfw.False)  // Start invisible, show later
	
	// Fixed window dimensions for reliability
	windowWidth := 800
	windowHeight := 600
	
	// Get available monitors
	glfwMonitors := glfw.GetMonitors()
	log.Printf("Found %d GLFW monitors", len(glfwMonitors))
	
	// Verify we have valid monitors
	if len(glfwMonitors) == 0 {
		return fmt.Errorf("no GLFW monitors found")
	}
	
	// Create windows one by one with processing events in between
	for i := uint32(0); i < c.localMonitors.MonitorCount; i++ {
		monitor := c.localMonitors.Monitors[i]
		log.Printf("Creating window %d of %d for monitor ID %d", i+1, c.localMonitors.MonitorCount, monitor.ID)
		
		// Process events to ensure GLFW is in a good state
		glfw.PollEvents()
		
		// Create window in windowed mode
		window, err := glfw.CreateWindow(
			windowWidth, 
			windowHeight,
			fmt.Sprintf("UltraRDP - Monitor %d", monitor.ID),
			nil, // No monitor association for windowed mode
			nil, // No shared context
		)
		
		// Handle creation errors
		if err != nil {
			log.Printf("ERROR: Failed to create window for monitor %d: %v", monitor.ID, err)
			// Continue to try creating other windows
			continue
		}
		
		// Calculate window position based on monitor position
		// Ensure each window has a unique position even if monitors overlap
		posX := int(monitor.PositionX)
		posY := int(monitor.PositionY)
		
		// Set position and store window
		window.SetPos(posX, posY)
		c.windows[i] = window
		
		// Make context current briefly to verify it works
		window.MakeContextCurrent()
		if err := gl.Init(); err != nil {
			log.Printf("WARNING: Failed to initialize OpenGL for window %d: %v", i, err)
		}
		// Release context
		glfw.DetachCurrentContext()
		
		// Show the window now that it's fully configured
		window.Show()
		
		// Process events after each window creation
		glfw.PollEvents()
		
		log.Printf("Window %d created successfully at position (%d, %d)", i+1, posX, posY)
	}
	
	// Count valid windows
	windowCount := 0
	for _, window := range c.windows {
		if window != nil {
			windowCount++
		}
	}
	
	if windowCount == 0 {
		return fmt.Errorf("failed to create any windows")
	}
	
	log.Printf("Created %d windows successfully", windowCount)
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
	defer glfw.Terminate()
	
	log.Printf("GLFW initialized successfully, version: %s", glfw.GetVersionString())
	
	// Create windows for each monitor
	if err := c.createWindows(); err != nil {
		log.Printf("ERROR: %v", err)
		return
	}
	
    // Initialize OpenGL and create textures
    textures := make([]uint32, len(c.windows))
    
    for i, window := range c.windows {
        if window == nil {
            continue
        }
        
        window.MakeContextCurrent()
        
        if err := gl.Init(); err != nil {
            log.Printf("Failed to initialize OpenGL for window %d: %v", i, err)
            continue
        }
        
        // Create and configure texture
        var texture uint32
        gl.GenTextures(1, &texture)
        gl.BindTexture(gl.TEXTURE_2D, texture)
        gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
        gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
        gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
        gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
        
        textures[i] = texture
        
        // Release context after initialization
        glfw.DetachCurrentContext()
        
    }

    log.Println("Entering display loop...")

    // Main display loop
    for !c.stopped {
        // Poll events
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
            log.Println("All windows closed, stopping client")
            c.Stop()
            break
        }
        
        // Update each window
        for i, window := range c.windows {
            if window == nil {
                log.Printf("Window %d is nil, skipping", i)
                continue
            }
            
            // Boundary check
            if i >= int(c.localMonitors.MonitorCount) {
                log.Printf("Window index %d out of range, skipping", i)
                continue
            }
            
            // Get monitor ID for this window
            monitorID := c.localMonitors.Monitors[i].ID
            
            // Get frame data with mutex protection
            c.frameMutex.Lock()
            frameData, exists := c.frameBuffers[monitorID]
            c.frameMutex.Unlock()
            
            if !exists || len(frameData) == 0 {
                // No frame data yet, just skip this window
                continue
            }
            
            // Make this window's context current
            window.MakeContextCurrent()
            
            // Validate JPEG header
            if len(frameData) < 2 || frameData[0] != 0xFF || frameData[1] != 0xD8 {
                log.Printf("Invalid JPEG data, skipping frame")
                continue
            }
            
            // Try to decode the JPEG with better error handling
            var img image.Image
            var err error
            
            func() {
                defer func() {
                    if r := recover(); r != nil {
                        log.Printf("Recovered from JPEG decode panic: %v", r)
                        err = fmt.Errorf("decode panic: %v", r)
                    }
                }()
                
                img, err = jpeg.Decode(bytes.NewReader(frameData))
            }()
            
            if err != nil {
                log.Printf("Error decoding JPEG: %v", err)
                
                // Create a red fallback image to show there's an error
                img = image.NewRGBA(image.Rect(0, 0, 320, 240))
                draw.Draw(img.(*image.RGBA), img.Bounds(), &image.Uniform{color.RGBA{255, 0, 0, 255}}, image.Point{}, draw.Src)
            }
            
            // Convert to RGBA
            bounds := img.Bounds()
            rgba := image.NewRGBA(bounds)
            if rgba.Stride != bounds.Dx()*4 {
                log.Printf("Unexpected stride: %d vs %d", rgba.Stride, bounds.Dx()*4)
            }
            
            draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)

            // Update texture
            gl.BindTexture(gl.TEXTURE_2D, textures[i])
            gl.TexImage2D(
                gl.TEXTURE_2D,
                0,
                gl.RGBA,
                int32(bounds.Dx()),
                int32(bounds.Dy()),
                0,
                gl.RGBA,
                gl.UNSIGNED_BYTE,
                gl.Ptr(rgba.Pix),
            )
            
            // Clear screen
            gl.ClearColor(0.0, 0.0, 0.0, 1.0)
            gl.Clear(gl.COLOR_BUFFER_BIT)

            // Set up orthographic projection
            gl.MatrixMode(gl.PROJECTION)
            gl.LoadIdentity()
            gl.Ortho(0, 1, 0, 1, -1, 1)

            // Set up model view
            gl.MatrixMode(gl.MODELVIEW)
            gl.LoadIdentity()

            // Enable texturing
            gl.Enable(gl.TEXTURE_2D)
            gl.BindTexture(gl.TEXTURE_2D, textures[i])

            // Draw a textured quad
            gl.Begin(gl.QUADS)
            // Bottom left
            gl.TexCoord2f(0, 0)
            gl.Vertex2f(0, 0)
            
            // Bottom right
            gl.TexCoord2f(1, 0)
            gl.Vertex2f(1, 0)
            
            // Top right
            gl.TexCoord2f(1, 1)
            gl.Vertex2f(1, 1)
            
            // Top left
            gl.TexCoord2f(0, 1)
            gl.Vertex2f(0, 1)
            gl.End()
            
            // Swap buffers
            window.SwapBuffers()
        }
    }
    
    // Clean up textures
    for i := range textures {
        if textures[i] != 0 {
            gl.DeleteTextures(1, &textures[i])
        }
    }

    log.Println("Display loop terminated, cleaning up resources")
    
    for _, window := range c.windows {
        if window != nil {
            window.Destroy()
        }
    }
}