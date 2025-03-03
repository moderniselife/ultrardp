package client

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"log"
	"runtime"
	"time"

	"github.com/go-gl/gl/v2.1/gl"  // Change to OpenGL 2.1 for maximum compatibility
	"github.com/go-gl/glfw/v3.3/glfw"
)

// createWindows creates GLFW windows for each mapped monitor
func (c *Client) createWindows() error {
	// Initialize windows slice
	c.windows = make([]*glfw.Window, c.localMonitors.MonitorCount)
    var windowsCreated uint32 = 0
    
    log.Printf("=== WINDOW CREATION START ===")
    log.Printf("Attempting to create %d windows for monitors (SIMPLIFIED VERSION)", c.localMonitors.MonitorCount)
	
	// Use most basic window creation settings
    for i := uint32(0); i < c.localMonitors.MonitorCount; i++ {
        monitor := c.localMonitors.Monitors[i]
        log.Printf("Creating window %d of %d for monitor %d", i+1, c.localMonitors.MonitorCount, monitor.ID)
        
        // Use fixed modest window size for compatibility
        windowWidth := 800
        windowHeight := 600
        
        // Use the simplest possible window hints for maximum compatibility
        glfw.DefaultWindowHints()
        glfw.WindowHint(glfw.ContextVersionMajor, 2)
        glfw.WindowHint(glfw.ContextVersionMinor, 1)
        // Don't specify any profile to use system default
        
        // Create window - always in windowed mode
        window, err := glfw.CreateWindow(
            windowWidth, 
            windowHeight,
            fmt.Sprintf("UltraRDP - Monitor %d", monitor.ID),
            nil,  // Always use windowed mode (no monitor association)
            nil,  // No shared context
        )
        
        if err != nil {
            log.Printf("Failed to create window: %v", err)
            continue
        }
        
        log.Printf("Window created successfully for monitor %d", monitor.ID)
        
        // Position window according to monitor layout
        window.SetPos(int(monitor.PositionX), int(monitor.PositionY))
        log.Printf("Window positioned at %d,%d", int(monitor.PositionX), int(monitor.PositionY))
        
        // Store window in slice
        c.windows[i] = window
        windowsCreated++
        
        // Process events and add delay between creations
        glfw.PollEvents()
        time.Sleep(200 * time.Millisecond)
    }
    
    // One final poll events
    glfw.PollEvents()
    
    // Check if we created at least one window
    if windowsCreated == 0 {
        return fmt.Errorf("failed to create any windows")
    }
    
    log.Printf("Successfully created %d of %d windows", windowsCreated, c.localMonitors.MonitorCount)
    log.Printf("=== WINDOW CREATION COMPLETE ===")
    return nil
}

// updateDisplayLoop handles the display loop for all monitors
func (c *Client) updateDisplayLoop() {
    // GLFW event handling must run on the main thread
    runtime.LockOSThread()

    // GLFW is already initialized in Start()
    defer glfw.Terminate()

    log.Printf("Starting display loop")

    // Create windows for each mapped monitor
    if err := c.createWindows(); err != nil {
        log.Printf("Failed to create windows: %v", err)
        // Continue despite errors to see if we get more diagnostic information
    }

    // Initialize OpenGL for each window and create resources
    log.Printf("=== INITIALIZING OPENGL ===")
    textures := make([]uint32, len(c.windows))
    successful := 0

    for i, window := range c.windows {
        if window == nil {
            log.Printf("Window %d is nil, skipping OpenGL initialization", i)
            continue
        }
        
        log.Printf("Initializing OpenGL for window %d", i)
        
        // Make this window's context current
        window.MakeContextCurrent()
        
        // Initialize OpenGL
        if err := gl.Init(); err != nil {
            log.Printf("Failed to initialize OpenGL for window %d: %v", i, err)
            continue
        }
        
        // Create texture for this window
        var texture uint32
        gl.GenTextures(1, &texture)
        textures[i] = texture
        gl.BindTexture(gl.TEXTURE_2D, texture)
        
        // Set texture parameters
        gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
        gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
        gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
        gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
        
        successful++
        log.Printf("Successfully initialized OpenGL for window %d", i)
    }
    
    log.Printf("Successfully initialized OpenGL for %d of %d windows", successful, len(c.windows))
    log.Printf("=== OPENGL INITIALIZATION COMPLETE ===")

    // Main display loop
    for !c.stopped {
        glfw.PollEvents()
        
        for i, window := range c.windows {            
            // Skip nil windows
            if window == nil {
                continue
            }
            
            // Check if window should close
            if window.ShouldClose() {
                c.Stop()
                break
            }

            // Verify the monitor index is valid
            if i >= int(c.localMonitors.MonitorCount) {
                continue // Skip this window
            }
            monitorID := c.localMonitors.Monitors[i].ID
            
            // Check if we have frame data for this monitor
            c.frameMutex.Lock()
            frameData, exists := c.frameBuffers[monitorID]
            c.frameMutex.Unlock()
            
            if !exists || len(frameData) == 0 {
                continue // Skip rendering if no frame data
            }
            
            // Make context current and render
            window.MakeContextCurrent()
            c.renderFrame(window, frameData, textures[i])
        }
        
        // Add a small sleep to prevent excessive CPU usage
        time.Sleep(16 * time.Millisecond) // ~60 FPS
    }

    // Cleanup
    for i := range textures {
        if textures[i] != 0 {
            gl.DeleteTextures(1, &textures[i])
        }
    }
    for _, window := range c.windows {
        if window != nil {
            window.Destroy()
        }
    }
}

// renderFrame renders a frame to the specified window
func (c *Client) renderFrame(window *glfw.Window, frameData []byte, texture uint32) {
    if len(frameData) == 0 {
        // Clear window if no frame data
        gl.ClearColor(0.0, 0.0, 0.0, 1.0) // Black background
        gl.Clear(gl.COLOR_BUFFER_BIT)
        window.SwapBuffers()
        return
    }
    
    // Validate JPEG format (check for SOI marker)
    if len(frameData) < 2 || frameData[0] != 0xFF || frameData[1] != 0xD8 {
        log.Printf("Error: Invalid JPEG format in renderFrame: missing SOI marker")
        // Clear window if frame data is invalid
        gl.ClearColor(0.0, 0.0, 0.0, 1.0)
        gl.Clear(gl.COLOR_BUFFER_BIT)
        window.SwapBuffers()
        return
    }

    // Decode JPEG frame data
    img, err := jpeg.Decode(bytes.NewReader(frameData))
    if err != nil {
        log.Printf("Error decoding JPEG frame: %v", err)
        return
    }

    // Convert image to RGBA
    bounds := img.Bounds()
    rgba := image.NewRGBA(bounds)
    draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)

    // Bind the texture
    gl.BindTexture(gl.TEXTURE_2D, texture)
    
    // Update texture with new frame data
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

    // Set up orthographic projection
    gl.MatrixMode(gl.PROJECTION)
    gl.LoadIdentity()
    gl.Ortho(0, 1, 0, 1, -1, 1)
    
    // Set up model view
    gl.MatrixMode(gl.MODELVIEW)
    gl.LoadIdentity()
    
    // Clear screen
    gl.ClearColor(0, 0, 0, 1)
    gl.Clear(gl.COLOR_BUFFER_BIT)
    
    // Enable texturing
    gl.Enable(gl.TEXTURE_2D)
    
    // Draw a textured quad
    gl.Begin(gl.QUADS)
    gl.TexCoord2f(0, 0)
    gl.Vertex2f(0, 0)
    
    gl.TexCoord2f(1, 0)
    gl.Vertex2f(1, 0)
    
    gl.TexCoord2f(1, 1)
    gl.Vertex2f(1, 1)
    
    gl.TexCoord2f(0, 1)
    gl.Vertex2f(0, 1)
    gl.End()
    
    // Disable texturing
    gl.Disable(gl.TEXTURE_2D)
    
    // Swap buffers
    window.SwapBuffers()
}