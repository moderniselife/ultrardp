package client

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"log"
	"runtime"

	"github.com/go-gl/gl/v3.3-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
)

// createWindows creates GLFW windows for each mapped monitor
func (c *Client) createWindows() error {
	// Initialize windows slice
	c.windows = make([]*glfw.Window, c.localMonitors.MonitorCount)
    var windowsCreated uint32 = 0
    
    log.Printf("Attempting to create %d windows for monitors", c.localMonitors.MonitorCount)
	// Get GLFW monitors
	monitors := glfw.GetMonitors()
	if len(monitors) == 0 {
		return fmt.Errorf("no monitors detected by GLFW")
	}

	// Set window hints
    log.Printf("GLFW version: %s", glfw.GetVersionString())
    log.Printf("Setting up window hints for %d GLFW monitors detected", len(monitors))
    
    // Reset window hints to default
    glfw.DefaultWindowHints()
    
    // Configure window properties
	glfw.WindowHint(glfw.Resizable, glfw.False)
	glfw.WindowHint(glfw.Decorated, glfw.False) // Borderless
    // Use OpenGL 3.3 instead of 4.1 for better compatibility
	glfw.WindowHint(glfw.ContextVersionMajor, 3)
	glfw.WindowHint(glfw.ContextVersionMinor, 3)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)

	// Create a window for each monitor
	for i := uint32(0); i < c.localMonitors.MonitorCount; i++ {
		monitor := c.localMonitors.Monitors[i]
		
		// Ensure we don't go out of bounds with monitors array
		var glfwMonitor *glfw.Monitor
		if int(i) < len(monitors) {
			glfwMonitor = monitors[i]
			log.Printf("Using GLFW monitor %d for local monitor %d", i, monitor.ID)
		} else {
			log.Printf("Warning: No matching GLFW monitor for local monitor %d, using nil", monitor.ID)
			glfwMonitor = nil // Use default monitor if no matching GLFW monitor
		}
		
		log.Printf("Creating window for monitor %d (%dx%d at %d,%d)", 
			monitor.ID, monitor.Width, monitor.Height, monitor.PositionX, monitor.PositionY)
		
        // Calculate window dimensions - cap width at 1920 for better compatibility with multi-monitor setups
        windowWidth := int(monitor.Width)
        if windowWidth > 1920 {
            windowWidth = 1920
            log.Printf("Limiting window width to 1920 pixels for better compatibility")
        }
        
		window, err := glfw.CreateWindow(
			windowWidth,
			int(monitor.Height),
			"UltraRDP",
			glfwMonitor, // Use GLFW monitor if available
			nil,
		)
		if err != nil {
			return fmt.Errorf("failed to create window for monitor %d: %v", monitor.ID, err)
		}

		// Set window position
		window.SetPos(int(monitor.PositionX), int(monitor.PositionY))
        
        // Process events to avoid GLFW overloading
        glfw.PollEvents()
        
        // Log success
        windowsCreated++
        log.Printf("Successfully created window %d of %d", windowsCreated, c.localMonitors.MonitorCount)

		// Store window
		c.windows[i] = window
        
        // Give GLFW time to process
	}

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
        // The normal return statement was here, but we'll continue execution to get more diagnostic logs
        // Removed: return
        log.Printf("GLFW monitors available: %d", len(glfw.GetMonitors()))
        log.Printf("Local monitors configured: %d", c.localMonitors.MonitorCount)
    }

    // Initialize OpenGL for each window and create resources
    textures := make([]uint32, len(c.windows))
    vaos := make([]uint32, len(c.windows))
    shaderPrograms := make([]uint32, len(c.windows))
    successful := 0

    for i, window := range c.windows {
        if window == nil {
            log.Printf("Warning: Window %d is nil, skipping OpenGL initialization", i)
            continue
        }
        
        log.Printf("Making context current for window %d", i)
        window.MakeContextCurrent()
        if err := gl.Init(); err != nil {
            log.Printf("Failed to initialize OpenGL 3.3 for window %d: %v", i, err)
            
            // Try with more compatible OpenGL version
            window.MakeContextCurrent()
            glfw.DefaultWindowHints()
            glfw.WindowHint(glfw.ContextVersionMajor, 2)
            glfw.WindowHint(glfw.ContextVersionMinor, 1)
            log.Printf("Attempting fallback to OpenGL 2.1 for better compatibility")
            continue // Skip this window and try the next one
        }

        // Create and bind texture
        gl.GenTextures(1, &textures[i])
        gl.BindTexture(gl.TEXTURE_2D, textures[i])

        // Set texture parameters
        gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
        gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
        gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
        gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)

        // Create vertex array object
        gl.GenVertexArrays(1, &vaos[i])
        gl.BindVertexArray(vaos[i])

        // Create vertex buffer
        vertices := []float32{
            // Position   // Texture coords
            -1.0, -1.0, 0.0, 0.0, // Bottom left
            1.0, -1.0, 1.0, 0.0,  // Bottom right
            -1.0, 1.0, 0.0, 1.0,  // Top left
            1.0, 1.0, 1.0, 1.0,   // Top right
        }

        var vbo uint32
        gl.GenBuffers(1, &vbo)
        gl.BindBuffer(gl.ARRAY_BUFFER, vbo)
        gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*4, gl.Ptr(vertices), gl.STATIC_DRAW)

        // Set vertex attributes
        gl.VertexAttribPointer(0, 2, gl.FLOAT, false, 4*4, gl.PtrOffset(0))
        gl.EnableVertexAttribArray(0)
        gl.VertexAttribPointer(1, 2, gl.FLOAT, false, 4*4, gl.PtrOffset(2*4))
        gl.EnableVertexAttribArray(1)

        // Create and compile shaders
        vertexShader := gl.CreateShader(gl.VERTEX_SHADER)
        vertexSource := `
            #version 330
            layout (location = 0) in vec2 position;
            layout (location = 1) in vec2 texCoord;
            out vec2 TexCoord;
            void main() {
                gl_Position = vec4(position, 0.0, 1.0);
                TexCoord = texCoord;
            }
        `
        csources, free := gl.Strs(vertexSource)
        gl.ShaderSource(vertexShader, 1, csources, nil)
        free()
        gl.CompileShader(vertexShader)

        fragmentShader := gl.CreateShader(gl.FRAGMENT_SHADER)
        fragmentSource := `
            #version 330
            in vec2 TexCoord;
            out vec4 FragColor;
            uniform sampler2D texture1;
            void main() {
                FragColor = texture(texture1, TexCoord);
            }
        `
        csources, free = gl.Strs(fragmentSource)
        gl.ShaderSource(fragmentShader, 1, csources, nil)
        free()
        gl.CompileShader(fragmentShader)

        // Create shader program
        shaderPrograms[i] = gl.CreateProgram()
        gl.AttachShader(shaderPrograms[i], vertexShader)
        gl.AttachShader(shaderPrograms[i], fragmentShader)
        gl.LinkProgram(shaderPrograms[i])
        gl.UseProgram(shaderPrograms[i])

        // Delete shaders as they're linked into the program and no longer necessary
        gl.DeleteShader(vertexShader)
        gl.DeleteShader(fragmentShader)
        
        successful++
        log.Printf("Successfully initialized OpenGL for window %d", i)
    }
    
    log.Printf("Successfully initialized %d of %d windows with OpenGL", successful, len(c.windows))

    // Main display loop
    for !c.stopped {
        c.frameMutex.Lock()
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

            // Get the monitor ID for this window
            if i >= int(c.localMonitors.MonitorCount) {
                log.Printf("Warning: Window index %d exceeds monitor count %d", i, c.localMonitors.MonitorCount)
                continue
            }
            monitorID := c.localMonitors.Monitors[i].ID
            
            // Check if we have frame data for this monitor
            frameData, exists := c.frameBuffers[monitorID]
            if !exists || len(frameData) == 0 {
                continue // Skip rendering if no frame data
            }
            
            // Make context current and render
            window.MakeContextCurrent()
            c.renderFrame(window, frameData, textures[i], vaos[i], shaderPrograms[i])
        }
        c.frameMutex.Unlock()
        
        // Process events
        glfw.PollEvents()
    }

    // Cleanup
    for i := range c.windows {
        gl.DeleteTextures(1, &textures[i])
        gl.DeleteVertexArrays(1, &vaos[i])
        gl.DeleteProgram(shaderPrograms[i])
    }
    for _, window := range c.windows {
        window.Destroy()
    }
}



// renderFrame renders a frame to the specified window
func (c *Client) renderFrame(window *glfw.Window, frameData []byte, texture, vao, shaderProgram uint32) {
    if len(frameData) == 0 {
        // Clear window if no frame data
        gl.ClearColor(0.0, 0.0, 0.0, 1.0) // Black background
        gl.Clear(gl.COLOR_BUFFER_BIT)
        window.SwapBuffers()
        return
    }
    
    log.Printf("Rendering frame with %d bytes of data", len(frameData))

    // Validate JPEG format (check for SOI marker)
    if len(frameData) < 2 || frameData[0] != 0xFF || frameData[1] != 0xD8 {
        log.Printf("Error: Invalid JPEG format in renderFrame: missing SOI marker")
        // Clear window if frame data is invalid
        gl.ClearColor(0.0, 0.0, 0.0, 1.0)
        gl.Clear(gl.COLOR_BUFFER_BIT)
        window.SwapBuffers()
        return
    }

    // Decode JPEG frame data - note that frameData is now raw JPEG data from the server
    // We no longer decode in updateFrameBuffer, only in renderFrame
    img, err := jpeg.Decode(bytes.NewReader(frameData))
    if err != nil {
        log.Printf("Error decoding JPEG frame: %v", err)
        return
    }

    // Convert image to RGBA
    bounds := img.Bounds()
    rgba := image.NewRGBA(bounds)
    draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)

    // Bind the texture and shader program
    gl.BindTexture(gl.TEXTURE_2D, texture)
    gl.UseProgram(shaderProgram)

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

    // Bind VAO
    gl.BindVertexArray(vao)

    // Clear and render
    gl.ClearColor(0.0, 0.0, 0.0, 1.0)
    gl.Clear(gl.COLOR_BUFFER_BIT)

    // Draw quad
    gl.DrawArrays(gl.TRIANGLE_STRIP, 0, 4)

    // Swap buffers
    window.SwapBuffers()
}