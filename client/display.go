package client

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"log"
	"runtime"

	"github.com/go-gl/gl/v2.1/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
)

// createWindows creates GLFW windows for each mapped monitor
func (c *Client) createWindows() error {
    // Initialize windows slice
    c.windows = make([]*glfw.Window, c.localMonitors.MonitorCount)
    
    // Set window creation hints - keeping it very simple
    glfw.DefaultWindowHints()  
    glfw.WindowHint(glfw.ContextVersionMajor, 2)
    glfw.WindowHint(glfw.ContextVersionMinor, 1)
    // Don't specify profile for OpenGL 2.1
    
    // Get available monitors
    monitors := glfw.GetMonitors()
    log.Printf("Found %d GLFW monitors", len(monitors))
    
    // Create windows
    for i := uint32(0); i < c.localMonitors.MonitorCount; i++ {
        monitor := c.localMonitors.Monitors[i]
        log.Printf("Creating window for monitor %d", monitor.ID)
        
        // Create window (always windowed mode for now)
        window, err := glfw.CreateWindow(
            800, 600, // Fixed size for simplicity
            fmt.Sprintf("UltraRDP - Monitor %d", monitor.ID),
            nil, // No monitor association
            nil, // No shared context
        )
        
        if err != nil {
            log.Printf("Failed to create window for monitor %d: %v", monitor.ID, err)
            continue
        }
        
        window.SetPos(int(monitor.PositionX), int(monitor.PositionY))
        c.windows[i] = window
    }
    
    // Count created windows
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
        
        textures[i] = texture
    }

    // Create a simple vertex buffer for rendering
    vertices := []float32{
        -1.0, -1.0, 0.0, 0.0, // bottom-left
        1.0, -1.0, 1.0, 0.0,  // bottom-right
        1.0, 1.0, 1.0, 1.0,   // top-right
        -1.0, 1.0, 0.0, 1.0,  // top-left
    }
    
    indices := []uint32{
        0, 1, 2, // first triangle
        2, 3, 0, // second triangle
    }
    
    // Create buffers
    var vbo, vao, ebo uint32
    gl.GenVertexArrays(1, &vao)
    gl.GenBuffers(1, &vbo)
    gl.GenBuffers(1, &ebo)
    
    gl.BindVertexArray(vao)
    
    gl.BindBuffer(gl.ARRAY_BUFFER, vbo)
    gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*4, gl.Ptr(vertices), gl.STATIC_DRAW)
    
    gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, ebo)
    gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(indices)*4, gl.Ptr(indices), gl.STATIC_DRAW)
    
    // Position attribute
    gl.VertexAttribPointer(0, 2, gl.FLOAT, false, 4*4, gl.PtrOffset(0))
    gl.EnableVertexAttribArray(0)
    
    // Texture coord attribute
    gl.VertexAttribPointer(1, 2, gl.FLOAT, false, 4*4, gl.PtrOffset(2*4))
    gl.EnableVertexAttribArray(1)
    
    // Main display loop
    for !c.stopped {
        // Poll events
        glfw.PollEvents()
        
        // Update each window
        for i, window := range c.windows {
            if window == nil {
                continue
            }
            
            // Check if window should close
            if window.ShouldClose() {
                c.Stop()
                break
            }
            
            // Skip if index is out of range
            if i >= int(c.localMonitors.MonitorCount) {
                continue
            }
            
            // Get monitor ID for this window
            monitorID := c.localMonitors.Monitors[i].ID
            
            // Get frame data (with mutex protection)
            c.frameMutex.Lock()
            frameData, exists := c.frameBuffers[monitorID]
            c.frameMutex.Unlock()
            
            if !exists || len(frameData) == 0 {
                continue
            }
            
            // Make context current
            window.MakeContextCurrent()
            
            // Skip if frame data is invalid
            if len(frameData) < 2 || frameData[0] != 0xFF || frameData[1] != 0xD8 {
                log.Printf("Invalid JPEG data, skipping frame")
                continue
            }
            
            // Decode JPEG
            img, err := jpeg.Decode(bytes.NewReader(frameData))
            if err != nil {
                log.Printf("Error decoding JPEG: %v", err)
                continue
            }
            
            // Convert to RGBA
            bounds := img.Bounds()
            rgba := image.NewRGBA(bounds)
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
    
    // Clean up resources
    gl.DeleteVertexArrays(1, &vao)
    gl.DeleteBuffers(1, &vbo)
    gl.DeleteBuffers(1, &ebo)
    
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