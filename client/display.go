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

	"github.com/go-gl/gl/v3.3-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
)

// createWindows creates GLFW windows for each mapped monitor
func (c *Client) createWindows() error {
	// Initialize windows slice
	c.windows = make([]*glfw.Window, c.localMonitors.MonitorCount)
    var windowsCreated uint32 = 0
    
    log.Printf("=== WINDOW CREATION START ===")
    log.Printf("Attempting to create %d windows for monitors", c.localMonitors.MonitorCount)
	
	// Get GLFW monitors
	monitors := glfw.GetMonitors()
	log.Printf("GLFW detected %d physical monitors", len(monitors))
	if len(monitors) == 0 {
		log.Printf("WARNING: No monitors detected by GLFW, using windowed mode")
	}

	// Set window hints using most compatible settings
    log.Printf("GLFW version: %s", glfw.GetVersionString())
    
    // Try to create windows one by one with increasing compatibility settings
    for i := uint32(0); i < c.localMonitors.MonitorCount; i++ {
        monitor := c.localMonitors.Monitors[i]
        log.Printf("Creating window %d of %d for monitor %d (%dx%d at %d,%d)", 
            i+1, c.localMonitors.MonitorCount, monitor.ID, 
            monitor.Width, monitor.Height, monitor.PositionX, monitor.PositionY)
        
        // Calculate window dimensions - cap width at 1280 for better compatibility
        windowWidth := int(monitor.Width)
        windowHeight := int(monitor.Height)
        
        if windowWidth > 1280 {
            windowWidth = 1280
            log.Printf("Limiting window width to 1280 pixels for better compatibility")
        }
        
        if windowHeight > 720 {
            windowHeight = 720
            log.Printf("Limiting window height to 720 pixels for better compatibility")
        }
        
        // Get corresponding GLFW monitor if available
        var glfwMonitor *glfw.Monitor = nil
        if int(i) < len(monitors) {
            glfwMonitor = monitors[i]
            log.Printf("Using physical GLFW monitor %d for logical monitor %d", i, monitor.ID)
        } else {
            log.Printf("No matching GLFW monitor for logical monitor %d, using windowed mode", monitor.ID)
        }
        
        // Try three different OpenGL versions in order of preference
        var window *glfw.Window = nil
        var err error
        
        // Try OpenGL 3.3 first (preferred)
        log.Printf("Attempting window creation with OpenGL 3.3")
        glfw.DefaultWindowHints()
        glfw.WindowHint(glfw.Resizable, glfw.False)
        glfw.WindowHint(glfw.Decorated, glfw.True) // Use decorated for better compatibility
        glfw.WindowHint(glfw.ContextVersionMajor, 3)
        glfw.WindowHint(glfw.ContextVersionMinor, 3)
        glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
        glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)
        
        // Try creating window for this monitor - use windowed mode for reliability
        window, err = glfw.CreateWindow(
            windowWidth,
            windowHeight,
            fmt.Sprintf("UltraRDP - Monitor %d", monitor.ID), 
            glfwMonitor, // Use the monitor we identified (can be nil)
            nil,
        )
        
        // If OpenGL 3.3 failed, try OpenGL 2.1 (backup)
        if err != nil {
            log.Printf("OpenGL 3.3 window creation failed: %v", err)
            log.Printf("Attempting fallback to OpenGL 2.1...")
            
            glfw.DefaultWindowHints()
            glfw.WindowHint(glfw.Resizable, glfw.False)
            glfw.WindowHint(glfw.Decorated, glfw.True)
            glfw.WindowHint(glfw.ContextVersionMajor, 2)
            glfw.WindowHint(glfw.ContextVersionMinor, 1)
            
            // Try again with OpenGL 2.1
            window, err = glfw.CreateWindow(
                windowWidth,
                windowHeight,
                fmt.Sprintf("UltraRDP - Monitor %d", monitor.ID),
                glfwMonitor,
                nil,
            )
        }
        
        // If still failed, try compatibility profile as last resort
        if err != nil {
            log.Printf("OpenGL 2.1 window creation failed: %v", err)
            log.Printf("Attempting last resort with compatibility profile...")
            
            glfw.DefaultWindowHints()
            glfw.WindowHint(glfw.Resizable, glfw.False)
            glfw.WindowHint(glfw.Decorated, glfw.True)
            glfw.WindowHint(glfw.ClientAPI, glfw.OpenGLAPI)
            glfw.WindowHint(glfw.ContextCreationAPI, glfw.NativeContextAPI)
            
            // Try one more time with compatibility profile
            window, err = glfw.CreateWindow(
                windowWidth,
                windowHeight,
                fmt.Sprintf("UltraRDP - Monitor %d", monitor.ID),
                glfwMonitor,
                nil,
            )
        }
        
        // Check if window creation failed after all attempts
        if err != nil {
            log.Printf("ERROR: All window creation attempts failed for monitor %d: %v", monitor.ID, err)
            continue // Skip this monitor and try the next one
        }
        
        // Window created successfully
        log.Printf("Successfully created window for monitor %d", monitor.ID)

        // Get the actual window dimensions and record for positioning
        var width, height int
        width, height = window.GetSize()
        log.Printf("Actual window dimensions: %dx%d", width, height)
        window.SetTitle(fmt.Sprintf("UltraRDP - Monitor %d (%dx%d)", monitor.ID, width, height))

        // Force window to be windowed mode (not fullscreen) for better positioning
        window.SetAttrib(glfw.Decorated, glfw.True)
        
        window.SetPos(int(monitor.PositionX), int(monitor.PositionY))
        log.Printf("Window position set to %d,%d", int(monitor.PositionX), int(monitor.PositionY))
        
        // Store window in slice
        c.windows[i] = window
        windowsCreated++
        
        // Process events after each window creation
        glfw.PollEvents()
        
        // Small delay to let GLFW process events
        time.Sleep(100 * time.Millisecond)
    }
    
    // Process events one more time after all windows are created
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
        log.Printf("GLFW monitors available: %d", len(glfw.GetMonitors()))
        log.Printf("Local monitors configured: %d", c.localMonitors.MonitorCount)
    }

    // Initialize OpenGL for each window and create resources
    log.Printf("=== INITIALIZING OPENGL ===")
    textures := make([]uint32, len(c.windows))
    vaos := make([]uint32, len(c.windows))
    shaderPrograms := make([]uint32, len(c.windows))
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
        
        // Create vertex array object
        var vao uint32
        gl.GenVertexArrays(1, &vao)
        vaos[i] = vao
        gl.BindVertexArray(vao)
        
        // Create vertex buffer
        vertices := []float32{
            // Position    // Texture coords
            -1.0, -1.0,    0.0, 0.0,  // Bottom left
            1.0, -1.0,     1.0, 0.0,  // Bottom right
            -1.0, 1.0,     0.0, 1.0,  // Top left
            1.0, 1.0,      1.0, 1.0,  // Top right
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
        
        // Create shader program
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
        
        // Check vertex shader compilation
        var success int32
        gl.GetShaderiv(vertexShader, gl.COMPILE_STATUS, &success)
        if success == gl.FALSE {
            var logLength int32
            gl.GetShaderiv(vertexShader, gl.INFO_LOG_LENGTH, &logLength)
            shaderLog := string(make([]byte, logLength+1))
            gl.GetShaderInfoLog(vertexShader, logLength, nil, gl.Str(shaderLog+"\x00"))
            log.Printf("Failed to compile vertex shader: %s", shaderLog)
            continue
        }
        
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
        
        // Check fragment shader compilation
        gl.GetShaderiv(fragmentShader, gl.COMPILE_STATUS, &success)
        if success == gl.FALSE {
            var logLength int32
            gl.GetShaderiv(fragmentShader, gl.INFO_LOG_LENGTH, &logLength)
            shaderLog := string(make([]byte, logLength+1))
            gl.GetShaderInfoLog(fragmentShader, logLength, nil, gl.Str(shaderLog+"\x00"))
            log.Printf("Failed to compile fragment shader: %s", shaderLog)
            continue
        }
        
        // Link shader program
        program := gl.CreateProgram()
        shaderPrograms[i] = program
        gl.AttachShader(program, vertexShader)
        gl.AttachShader(program, fragmentShader)
        gl.LinkProgram(program)
        
        // Check program linking
        gl.GetProgramiv(program, gl.LINK_STATUS, &success)
        if success == gl.FALSE {
            var logLength int32
            gl.GetProgramiv(program, gl.INFO_LOG_LENGTH, &logLength)
            programLog := string(make([]byte, logLength+1))
            gl.GetProgramInfoLog(program, logLength, nil, gl.Str(programLog+"\x00"))
            log.Printf("Failed to link shader program: %s", programLog)
            continue
        }
        
        gl.UseProgram(program)
        
        // Delete shaders after linking
        gl.DeleteShader(vertexShader)
        gl.DeleteShader(fragmentShader)
        
        successful++
        log.Printf("Successfully initialized OpenGL for window %d", i)
    }
    
    log.Printf("Successfully initialized OpenGL for %d of %d windows", successful, len(c.windows))
    log.Printf("=== OPENGL INITIALIZATION COMPLETE ===")

    // Function to check and update window positions
    updateWindowPositions := func() {
        for i, window := range c.windows {
            if window == nil || i >= int(c.localMonitors.MonitorCount) {
                continue
            }
            
            // Get the monitor info for this window
            monitor := c.localMonitors.Monitors[i]
            
            // Get current window position
            x, y := window.GetPos()
            
            // Update position if it doesn't match monitor position
            if x != int(monitor.PositionX) || y != int(monitor.PositionY) {
                log.Printf("Repositioning window %d to %d,%d (was at %d,%d)", i, monitor.PositionX, monitor.PositionY, x, y)
                window.SetPos(int(monitor.PositionX), int(monitor.PositionY))
            }
        }
    }
    // Main display loop
    for !c.stopped {
        glfw.PollEvents()
        
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

            // Verify the monitor index is valid
            if i >= int(c.localMonitors.MonitorCount) {
                log.Printf("Warning: Window index %d exceeds monitor count %d", i, c.localMonitors.MonitorCount)
                continue // Skip this window
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
        
        // Check window positions every 30 frames
        updateWindowPositions()
    }
    
    // Clean up resources before termination
    log.Printf("Cleaning up resources...")
    gl.Finish()
    glfw.PollEvents()

    // Cleanup
    for i := range c.windows {
        gl.DeleteTextures(1, &textures[i])
        gl.DeleteVertexArrays(1, &vaos[i])
        gl.DeleteProgram(shaderPrograms[i])
    }
    for _, window := range c.windows {
        if window != nil {
            window.Destroy()
        }
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
    
    // Print header bytes to debug the JPEG data
    headerStr := ""
    for i := 0; i < min(16, len(frameData)); i++ {
        headerStr += fmt.Sprintf("%02X ", frameData[i])
    }
    log.Printf("JPEG header bytes: %s", headerStr)
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

    // Decode JPEG frame data
    img, err := jpeg.Decode(bytes.NewReader(frameData))
    if err != nil {
        log.Printf("Error decoding JPEG frame: %v", err)
        return
    } else {
        // Log successful decoding
        log.Printf("Successfully decoded JPEG frame: %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
    }

    // Convert image to RGBA
    bounds := img.Bounds()
    rgba := image.NewRGBA(bounds)
    draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)

    // Bind the texture and shader program
    var glErr uint32
    
    // Check for OpenGL errors before binding
    if glErr = gl.GetError(); glErr != gl.NO_ERROR {
        log.Printf("OpenGL error before binding texture: 0x%x", glErr)
    }
    
    gl.BindTexture(gl.TEXTURE_2D, texture)
    
    if texture == 0 {
        log.Printf("Error: Invalid texture ID 0")
        return
    }
    
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
    
    // Check for OpenGL errors after texture update
    if glErr = gl.GetError(); glErr != gl.NO_ERROR {
        log.Printf("OpenGL error after updating texture: 0x%x", glErr)
    }

    // Bind VAO
    gl.BindVertexArray(vao)
    if vao == 0 {
        log.Printf("Error: Invalid VAO ID 0")
        return
    }

    // Clear and render
    gl.ClearColor(0.0, 0.0, 0.0, 1.0)
    gl.Clear(gl.COLOR_BUFFER_BIT)

    // Draw quad
    log.Printf("Drawing quad with texture %d, vao %d, shader %d", texture, vao, shaderProgram)
    gl.DrawArrays(gl.TRIANGLE_STRIP, 0, 4)

    // Swap buffers
    window.SwapBuffers()
    gl.Finish() // Ensure all OpenGL commands are completed
}

// Helper function to find minimum of two ints
func min(a, b int) int {
    if a < b { return a } else { return b }
}