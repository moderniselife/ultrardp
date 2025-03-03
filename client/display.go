package client

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"log"
	"runtime"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
)

// createWindows creates GLFW windows for each mapped monitor
func (c *Client) createWindows() error {
	// Initialize windows slice
	c.windows = make([]*glfw.Window, c.localMonitors.MonitorCount)

	// Get GLFW monitors
	monitors := glfw.GetMonitors()
	if len(monitors) == 0 {
		return fmt.Errorf("no monitors detected by GLFW")
	}

	// Set window hints
	glfw.WindowHint(glfw.Resizable, glfw.False)
	glfw.WindowHint(glfw.Decorated, glfw.False) // Borderless
	glfw.WindowHint(glfw.ContextVersionMajor, 4)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)

	// Create a window for each monitor
	for i := uint32(0); i < c.localMonitors.MonitorCount; i++ {
		monitor := c.localMonitors.Monitors[i]
		// Create window
		window, err := glfw.CreateWindow(
			int(monitor.Width),
			int(monitor.Height),
			"UltraRDP",
			monitors[i], // Use corresponding GLFW monitor
			nil,
		)
		if err != nil {
			return fmt.Errorf("failed to create window for monitor %d: %v", monitor.ID, err)
		}

		// Set window position
		window.SetPos(int(monitor.PositionX), int(monitor.PositionY))

		// Store window
		c.windows[i] = window
	}

	return nil
}



// updateDisplayLoop handles the display loop for all monitors
func (c *Client) updateDisplayLoop() {
    // GLFW event handling must run on the main thread
    runtime.LockOSThread()

    // GLFW is already initialized in Start()
    defer glfw.Terminate()

    // Create windows for each mapped monitor
    if err := c.createWindows(); err != nil {
        log.Printf("Failed to create windows: %v", err)
        return
    }

    // Initialize OpenGL for each window and create resources
    textures := make([]uint32, len(c.windows))
    vaos := make([]uint32, len(c.windows))
    shaderPrograms := make([]uint32, len(c.windows))

    for i, window := range c.windows {
        window.MakeContextCurrent()
        if err := gl.Init(); err != nil {
            log.Printf("Failed to initialize OpenGL: %v", err)
            return
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
            #version 410
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
            #version 410
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

            // Render the frame using the corresponding resources
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