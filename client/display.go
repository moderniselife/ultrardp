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

	// Initialize OpenGL for each window and create resources
	for _, window := range c.windows {
		window.MakeContextCurrent()
		if err := gl.Init(); err != nil {
			log.Printf("Failed to initialize OpenGL: %v", err)
			return
		}

		// Create and bind texture
		var texture uint32
		gl.GenTextures(1, &texture)
		gl.BindTexture(gl.TEXTURE_2D, texture)

		// Set texture parameters
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)

		// Create vertex array object
		var vao uint32
		gl.GenVertexArrays(1, &vao)
		gl.BindVertexArray(vao)

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
		shaderProgram := gl.CreateProgram()
		gl.AttachShader(shaderProgram, vertexShader)
		gl.AttachShader(shaderProgram, fragmentShader)
		gl.LinkProgram(shaderProgram)
		gl.UseProgram(shaderProgram)
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
    if len(frameData) == 0 {
        // Clear window if no frame data
        gl.ClearColor(0.0, 0.0, 0.0, 1.0) // Black background
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

    // Clear and render
    gl.ClearColor(0.0, 0.0, 0.0, 1.0)
    gl.Clear(gl.COLOR_BUFFER_BIT)

    // Draw quad
    gl.DrawArrays(gl.TRIANGLE_STRIP, 0, 4)

    // Swap buffers
    window.SwapBuffers()}