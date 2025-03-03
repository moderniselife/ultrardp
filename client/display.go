package client

import (
	"fmt"
	"time"
	"os"
	"path/filepath"
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	_ "image/png"
	_ "image/jpeg"
	"image/draw"

	"github.com/go-gl/gl/v2.1/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
)

// Create a debug directory for saving frames
func createDebugDir(dir string) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintf(os.Stdout, "Failed to create debug directory: %v\n", err)
	}
}

// Save image to file for debugging
func saveImageToFile(img image.Image, monitorID uint32, frameNum int, format string) string {
	debugDir := "debug_frames"
	createDebugDir(debugDir)
	
	filename := filepath.Join(debugDir, fmt.Sprintf("decoded_mon%d_%d.%s", monitorID, frameNum, format))
	f, err := os.Create(filename)
	if err != nil {
		fmt.Fprintf(os.Stdout, "Error creating debug file: %v\n", err)
		return ""
	}
	defer f.Close()
	
	if format == "jpg" {
		jpeg.Encode(f, img, &jpeg.Options{Quality: 90})
	} else if format == "png" {
		png.Encode(f, img)
	}
	
	fmt.Fprintf(os.Stdout, "Saved decoded image to %s\n", filename)
	return filename
}

// renderSimpleFullscreenTexture renders a texture using the simplest possible approach
func renderSimpleFullscreenTexture(textureID uint32) {
	// Reset OpenGL state completely
	gl.GetError() // Clear any previous errors
	
	// Disable everything that could interfere
	gl.Disable(gl.DEPTH_TEST)
	gl.Disable(gl.CULL_FACE)
	gl.Disable(gl.BLEND)
	gl.Disable(gl.LIGHTING)
	
	// Set up a simple orthographic projection
	gl.MatrixMode(gl.PROJECTION)
	gl.LoadIdentity()
	gl.Ortho(0, 1, 0, 1, -1, 1)
	
	gl.MatrixMode(gl.MODELVIEW)
	gl.LoadIdentity()
	
	// Enable texturing
	gl.Enable(gl.TEXTURE_2D)
	
	// Bind the texture and set parameters
	gl.BindTexture(gl.TEXTURE_2D, textureID)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	
	// Set color to pure white (1,1,1,1) to show texture as-is
	gl.Color4f(1.0, 1.0, 1.0, 1.0)
	
	// Draw a fullscreen quad with the texture - with standard orientation
	gl.Begin(gl.QUADS)
	gl.TexCoord2f(0.0, 0.0); gl.Vertex2f(0.0, 0.0) // Bottom-left
	gl.TexCoord2f(1.0, 0.0); gl.Vertex2f(1.0, 0.0) // Bottom-right
	gl.TexCoord2f(1.0, 1.0); gl.Vertex2f(1.0, 1.0) // Top-right
	gl.TexCoord2f(0.0, 1.0); gl.Vertex2f(0.0, 1.0) // Top-left
	gl.End()
	
	// Disable texturing when done
	gl.Disable(gl.TEXTURE_2D)
}

// createWindows creates a window for each monitor
func (c *Client) createWindows() error {
	fmt.Println("Creating windows for RDP client...")
	
	// Get information about available monitors directly from GLFW
	monitors := glfw.GetMonitors()
	fmt.Printf("Found %d GLFW monitors\n", len(monitors))
	
	// Print detailed monitor info
	for i, monitor := range monitors {
		x, y := monitor.GetPos()
		mode := monitor.GetVideoMode()
		fmt.Printf("Monitor %d: %s at (%d,%d) resolution %dx%d\n", 
			i, monitor.GetName(), x, y, mode.Width, mode.Height)
		
		// Detect and fix invalid coordinates
		if x < -10000 || x > 10000 || y < -10000 || y > 10000 {
			fmt.Printf("WARNING: Monitor %d has suspicious coordinates (%d,%d), will use fallback position\n", 
				i, x, y)
		}
	}
	
	// Initialize windows slice - use GLFW monitor count
	monitorCount := len(monitors)
	fmt.Printf("Creating %d windows\n", monitorCount)
	c.windows = make([]*glfw.Window, monitorCount)
	
	// Create textures - this will be populated later
	textures := make(map[int]uint32)
	
	// Create a window for each monitor (following the working example's approach)
	for i, monitor := range monitors {
		fmt.Printf("Creating window %d for monitor %s\n", i, monitor.GetName())
		
		// Window creation hints 
		glfw.DefaultWindowHints()
		glfw.WindowHint(glfw.Visible, glfw.True)
		glfw.WindowHint(glfw.Decorated, glfw.True)
		glfw.WindowHint(glfw.Resizable, glfw.False)
		glfw.WindowHint(glfw.ContextVersionMajor, 2)
		glfw.WindowHint(glfw.ContextVersionMinor, 1)
		glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLAnyProfile)
		
		// Get monitor dimensions
		mode := monitor.GetVideoMode()
		x, y := monitor.GetPos()
		
		// Fixed window size for debugging
		width, height := 800, 600
		
		// Create window - using exact same approach as the working example
		window, err := glfw.CreateWindow(
			width, height,
			fmt.Sprintf("UltraRDP - Monitor %d", i),
			nil, nil)
		
		if err != nil {
			fmt.Printf("Failed to create window for monitor %d: %v\n", i, err)
			continue
		}
		
		// Position window on monitor
		if x >= -10000 && x <= 10000 && y >= -10000 && y <= 10000 {
			centerX := x + (mode.Width - width) / 2
			centerY := y + (mode.Height - height) / 2
			fmt.Printf("Window %d position: %d,%d\n", i, centerX, centerY)
			window.SetPos(centerX, centerY)
		} else {
			// Fallback position for suspicious coordinates
			fmt.Printf("Using fallback positioning for window %d\n", i)
			switch i {
			case 0:
				window.SetPos(100, 100)
			case 1: 
				window.SetPos(300, 300)
			case 2:
				window.SetPos(500, 500)
			default:
				window.SetPos(100+i*200, 100+i*200)
			}
		}
		
		// Store the window
		c.windows[i] = window
		
		// Make sure the window is visible
		window.Show()
		fmt.Printf("Window %d created and shown\n", i)
		
		// Process events immediately after creation
		glfw.PollEvents()
		
		// Add delay between window creations
		time.Sleep(100 * time.Millisecond)
	}
	
	// Make first window's context current for OpenGL initialization
	if len(c.windows) > 0 && c.windows[0] != nil {
		c.windows[0].MakeContextCurrent()
		
		// Initialize OpenGL
		if err := gl.Init(); err != nil {
			fmt.Printf("Failed to initialize OpenGL: %v\n", err)
			return err
		}
		
		fmt.Printf("OpenGL initialized: %s\n", gl.GoStr(gl.GetString(gl.VERSION)))
		
		// Create a texture for each window
		for i, window := range c.windows {
			if window == nil {
				continue
			}
			
			window.MakeContextCurrent()
			var texture uint32
			gl.GenTextures(1, &texture)
			textures[i] = texture
			fmt.Printf("Created texture %d for window %d\n", texture, i)
		}
	} else {
		return fmt.Errorf("no valid windows created")
	}
	
	// Count how many windows were successfully created
	windowCount := 0
	for _, w := range c.windows {
		if w != nil {
			windowCount++
		}
	}
	
	fmt.Printf("Successfully created %d windows\n", windowCount)
	
	if windowCount == 0 {
		return fmt.Errorf("failed to create any windows")
	}
	
	return nil
}

// displayFrame displays a JPEG frame in the given window
func (c *Client) displayFrame(windowIndex int, frameData []byte, frameNumber int) error {
	// Ensure we have the correct window context
	window := c.windows[windowIndex]
	if window == nil || window.ShouldClose() {
		return fmt.Errorf("window %d is nil or should close", windowIndex)
	}
	
	// Make window current
	window.MakeContextCurrent()
	
	// Try to decode the JPEG frame
	img, err := jpeg.Decode(bytes.NewReader(frameData))
	if err != nil {
		fmt.Printf("Error decoding JPEG for window %d: %v\n", windowIndex, err)
		
		// Save the raw JPEG data for analysis
		rawFrameFile := filepath.Join("debug_frames", fmt.Sprintf("raw_frame_win%d.jpg", windowIndex))
		if err := os.WriteFile(rawFrameFile, frameData, 0644); err != nil {
			fmt.Printf("Error saving raw frame data: %v\n", err)
		} else {
			fmt.Printf("Saved raw JPEG data to %s\n", rawFrameFile)
		}
		
		return err
	}
	
	// Get local monitor ID and find the corresponding server monitor ID
	localMonID := uint32(windowIndex + 1)
	
	// Save decoded image for debugging
	saveImageToFile(img, localMonID, frameNumber, "jpg")
	
	// Convert to RGBA
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Over)
	
	// Create or get texture
	var texture uint32
	gl.GenTextures(1, &texture)
	
	// Bind the texture
	gl.BindTexture(gl.TEXTURE_2D, texture)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	
	// Force 1-byte alignment for any image
	gl.PixelStorei(gl.UNPACK_ALIGNMENT, 1)
	
	// Upload texture
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
	
	// Clear the background
	gl.ClearColor(0.2, 0.2, 0.2, 1.0)
	gl.Clear(gl.COLOR_BUFFER_BIT)
	
	// Render the texture
	renderSimpleFullscreenTexture(texture)
	
	// Cleanup
	gl.DeleteTextures(1, &texture)
	
	return nil
}

// updateDisplayLoop handles the display loop for all monitors
func (c *Client) updateDisplayLoop() {
	fmt.Fprintln(os.Stdout, "*** Starting display loop using GLFW ***")
	
	// Initialize GLFW first - this must be done on the main thread
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
	
	// Create debug directory
	createDebugDir("debug_frames")
	
	// Variables for monitoring
	frameCount := 0
	lastFPSTime := time.Now()
	framesRendered := 0
	
	// Main display loop - following the cmd_client.go approach
	fmt.Fprintln(os.Stdout, "Starting main display loop")
	for !c.stopped {
		frameCount++
		
		// Process window events
		glfw.PollEvents()
		
		// Check for window close events
		allClosed := true
		for _, window := range c.windows {
			if window != nil && !window.ShouldClose() {
				allClosed = false
				break
			}
		}
		
		if allClosed {
			fmt.Println("All windows closed")
			c.stopped = true
			break
		}
		
		// Render each window
		for windowIndex, window := range c.windows {
			if window == nil {
				continue
			}
			
			// Skip if window should close
			if window.ShouldClose() {
				continue
			}
			
			// Get the server monitor ID for this window
			localMonID := uint32(windowIndex + 1)
			serverMonID := uint32(0)
			
			// Find the server monitor ID mapped to this local monitor
			for srvID, locID := range c.monitorMap {
				if locID == localMonID {
					serverMonID = srvID
					break
				}
			}
			
			if serverMonID == 0 {
				// Only log this occasionally to avoid spam
				if frameCount % 30 == 0 {
					fmt.Printf("Window %d not mapped to any server monitor\n", windowIndex)
				}
				continue
			}
			
			// Check if we have frame data for this monitor
			c.frameMutex.Lock()
			frameData, exists := c.frameBuffers[localMonID]
			
			if !exists || len(frameData) == 0 {
				// Only log this occasionally
				if frameCount % 30 == 0 {
					fmt.Printf("Window %d mapped to server monitor %d, frame exists: %v\n", 
						windowIndex, serverMonID, exists && len(frameData) > 0)
					fmt.Printf("No frame data for window %d (server monitor %d)\n", 
						windowIndex, serverMonID)
				}
				c.frameMutex.Unlock()
				
				// Make the window current and draw a blue background
				window.MakeContextCurrent()
				gl.ClearColor(0.0, 0.0, 0.2, 1.0) // Dark blue 
				gl.Clear(gl.COLOR_BUFFER_BIT)
				window.SwapBuffers()
				
				continue
			}
			
			// Make a copy of the frame data
			frameDataCopy := make([]byte, len(frameData))
			copy(frameDataCopy, frameData)
			c.frameMutex.Unlock()
			
			// Display the frame
			err := c.displayFrame(windowIndex, frameDataCopy, frameCount)
			if err != nil {
				fmt.Printf("Error rendering frame: %v\n", err)
			}
			
			// Swap buffers
			window.SwapBuffers()
			framesRendered++
		}
		
		// Calculate and display FPS occasionally
		if time.Since(lastFPSTime) >= time.Second {
			fps := float64(framesRendered) / time.Since(lastFPSTime).Seconds()
			fmt.Printf("FPS: %.2f\n", fps)
			framesRendered = 0
			lastFPSTime = time.Now()
		}
		
		// Small sleep to prevent high CPU usage
		time.Sleep(33 * time.Millisecond) // ~30fps
	}
	
	fmt.Fprintln(os.Stdout, "Display loop terminated")
}