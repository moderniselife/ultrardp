package main

import (
	"fmt"
	"bytes"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/go-gl/gl/v2.1/gl"

	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/moderniselife/ultrardp/protocol"
)

type SimpleClient struct {
	conn           net.Conn
	serverMonitors *protocol.MonitorConfig
	localMonitors  *protocol.MonitorConfig
	stopped        bool
	stopChan       chan struct{}
	frameMutex     sync.Mutex
	frameBuffers   map[uint32][]byte
	frameCount     map[uint32]int
	windows        []*glfw.Window
	textures       map[int]uint32  // Window index to texture ID
	monitorMap     map[uint32]int  // Server monitor ID to window index
}

func main() {
	// Force display code to run on the main thread
	runtime.LockOSThread()

	// Parse command line arguments
	
	serverAddr := "macbook.tail85acc2.ts.net:8000"
	if len(os.Args) > 1 {
		serverAddr = os.Args[1]
	}
	
	fmt.Printf("=== UltraRDP simplified client connecting to %s ===\n", serverAddr)
	
	// Create client
	client := &SimpleClient{
		textures:     make(map[int]uint32),
		stopChan:    make(chan struct{}),
		frameBuffers: make(map[uint32][]byte),
		frameCount:   make(map[uint32]int),
	}
	
	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("Received termination signal")
		client.Stop()
	}()
	
	// Connect to server
	fmt.Println("Connecting to server...")
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	client.conn = conn
	defer conn.Close()
	
	// Initialize GLFW early
	if err := glfw.Init(); err != nil {
		log.Fatalf("Failed to initialize GLFW: %v", err)
	}
	defer glfw.Terminate()
	
	fmt.Printf("GLFW initialized successfully, version: %s\n", glfw.GetVersionString())
	
	// Detect monitors
	monitors := glfw.GetMonitors()
	fmt.Printf("Found %d monitors\n", len(monitors))
	
	// Setup monitor config
	localMonitors := &protocol.MonitorConfig{
		MonitorCount: uint32(len(monitors)),
		Monitors:     make([]protocol.MonitorInfo, len(monitors)),
	}
	
	for i, monitor := range monitors {
		mode := monitor.GetVideoMode()
		x, y := monitor.GetPos()
		
		localMonitors.Monitors[i] = protocol.MonitorInfo{
			ID:        uint32(i + 1),
			Width:     uint32(mode.Width),
			Height:    uint32(mode.Height),
			// Converting to uint32 because protocol.MonitorInfo expects these as unsigned
			PositionX: uint32(x),
			PositionY: uint32(y),
			Primary:   i == 0,
		}
		
		fmt.Printf("Monitor %d: %s at (%d,%d) resolution %dx%d\n", 
			i, monitor.GetName(), x, y, mode.Width, mode.Height)
	}
	fmt.Println("=================================================")
	
	client.localMonitors = localMonitors
	
	// Start network handler in background
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		client.networkHandler()
	}()
	
	// Create windows and prepare for rendering
	client.createWindows()
	
	fmt.Println("=================================================")
	// Main display loop
	fmt.Println("Starting main display loop with monitor mappings:", client.monitorMap)
	for !client.stopped {
		// Poll for GLFW events
		glfw.PollEvents()
		
		// Render frames to each window
		for i, window := range client.windows {
			if window == nil {
				continue
			}
			
			// Skip if window should close
			if window.ShouldClose() {
				continue
			}
			
			// Get the server monitor ID for this window (simple 1:1 mapping for now)
			var serverMonitorID uint32
			
			// Look for this window index in the monitor map
			for sID, wIdx := range client.monitorMap {
				if wIdx == i {
					serverMonitorID = sID
				}
				if sID-1 == uint32(i) {
					serverMonitorID = sID
				}
			}
			
			// Get the frame data for this monitor
			client.frameMutex.Lock()
			frameData, exists := client.frameBuffers[serverMonitorID]
			client.frameMutex.Unlock()
			fmt.Printf("Window %d mapped to server monitor %d, frame exists: %v\n", i, serverMonitorID, exists)

			if exists && len(frameData) > 0 {
				window.MakeContextCurrent()

				// Ensure texture exists for this window
				if _, ok := client.textures[i]; !ok {
					client.textures[i] = client.createTexture()
				}
				
				fmt.Printf("Rendering frame for monitor %d to window %d (%d bytes)\n", 
					serverMonitorID, i, len(frameData))
				
				// Display the frame
				time.Sleep(50 * time.Millisecond) // Give some time for context switching
				err := client.displayFrame(i, frameData)
				if err != nil {
					fmt.Printf("Error rendering frame: %v\n", err)
				}
				
				window.SwapBuffers()
			} else {
				// Even if no frame, make the window current and clear it to show something
				window.MakeContextCurrent()
				gl.ClearColor(0.0, 0.0, 0.2, 1.0) // Dark blue 
				gl.Clear(gl.COLOR_BUFFER_BIT)
				
				window.SwapBuffers()
				
				fmt.Printf("No frame data for window %d (server monitor %d)\n", i, serverMonitorID)
			}
		}
		
		// Process window events
		// Process window events and check for closed windows
		allClosed := true
		for _, window := range client.windows {
			if window != nil && !window.ShouldClose() {
				allClosed = false
				break
			}
		}
		
		if allClosed {
			fmt.Println("All windows closed")
			client.Stop()
		}
		
		// Small sleep to prevent high CPU usage
		time.Sleep(33 * time.Millisecond) // ~30fps
	}
	
	// Wait for network handler to finish
	wg.Wait()
	fmt.Println("Client terminated successfully")
}

// Stop signals the client to stop
func (c *SimpleClient) Stop() {
	if !c.stopped {
		c.stopped = true
		close(c.stopChan)
	}
}

// createWindows creates a window for each monitor
func (c *SimpleClient) createWindows() {
	fmt.Println("Creating windows...")
	
	// Initialize the monitor map
	c.monitorMap = make(map[uint32]int)
	
	monitors := glfw.GetMonitors()
	c.windows = make([]*glfw.Window, len(monitors))
	
	for i, monitor := range monitors {
		fmt.Printf("Creating window %d for monitor %s\n", i, monitor.GetName())
		
		// Window creation hints 
		glfw.DefaultWindowHints()
		glfw.WindowHint(glfw.Visible, glfw.True)
		glfw.WindowHint(glfw.Decorated, glfw.True)
		glfw.WindowHint(glfw.Resizable, glfw.False)
		
		// Get monitor dimensions
		mode := monitor.GetVideoMode()
		x, y := monitor.GetPos()
		
		// Fixed window size for debugging
		width, height := 800, 600
		
		// Create window
		window, err := glfw.CreateWindow(
			width, height,
			fmt.Sprintf("UltraRDP - Monitor %d", i),
			nil, nil)
		
		if err != nil {
			fmt.Printf("Failed to create window for monitor %d: %v\n", i, err)
			continue
		}
		
		// Position window on monitor
		centerX := x + (mode.Width - width) / 2
		centerY := y + (mode.Height - height) / 2
		fmt.Printf("Window %d position: %d,%d\n", i, centerX, centerY)
		window.SetPos(centerX, centerY)
		
		// Make sure the window is visible
		window.Show()
		
		// Make window's context current for OpenGL init
		window.MakeContextCurrent()
		
		// Initialize OpenGL for this window
		if i == 0 { // Only initialize OpenGL once
			if err := gl.Init(); err != nil {
				fmt.Printf("Failed to initialize OpenGL: %v\n", err)
				continue
			}
		}
		
		// Create a texture for this window and store it
		texture := c.initializeTexture()
		c.textures[i] = texture
		
		// Finish window creation
		window.SetPos(centerX, centerY)
		window.Show()
		
		c.windows[i] = window
		fmt.Printf("Window %d created successfully\n", i)
		
		// Process events immediately
		glfw.PollEvents()
		
		// Add delay between window creations
		time.Sleep(100 * time.Millisecond)
	}
}

// createTexture creates a new OpenGL texture
func (c *SimpleClient) createTexture() uint32 {
	var texture uint32
	gl.GenTextures(1, &texture)
	gl.BindTexture(gl.TEXTURE_2D, texture)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	return texture
}

// initializeTexture creates an OpenGL texture
func (c *SimpleClient) initializeTexture() uint32 {
	var texture uint32
	gl.GenTextures(1, &texture)
	gl.BindTexture(gl.TEXTURE_2D, texture)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	return texture
}

// saveImageToFile saves an image to a file
func saveImageToFile(img image.Image, monitorID uint32, frameNum int, format string) string {
	// Create debug directory if it doesn't exist
	debugDir := "debug_frames"
	if err := os.MkdirAll(debugDir, 0755); err != nil {
		fmt.Printf("Error creating debug directory: %v\n", err)
		return ""
	}
	
	// Create a filename with monitor ID and frame number
	var filename string
	var f *os.File
	var err error
	
	if format == "png" {
		filename = filepath.Join(debugDir, fmt.Sprintf("frame_mon%d_%d.png", monitorID, frameNum))
		f, err = os.Create(filename)
		if err != nil {
			fmt.Printf("Error creating debug file: %v\n", err)
			return ""
		}
		defer f.Close()
		png.Encode(f, img)
	} else {
		filename = filepath.Join(debugDir, fmt.Sprintf("frame_mon%d_%d.jpg", monitorID, frameNum))
		f, err = os.Create(filename)
		if err != nil {
			fmt.Printf("Error creating debug file: %v\n", err)
			return ""
		}
		defer f.Close()
		jpeg.Encode(f, img, nil)
	}
	
	return filename
}

// renderFrame renders a JPEG frame to the given window
func (c *SimpleClient) renderFrame(windowIndex int, frameData []byte) error {
	// Ensure we have the correct window context before anything else
	window := c.windows[windowIndex]
	if window == nil {
		return fmt.Errorf("window %d is nil", windowIndex)
	}
	window.MakeContextCurrent()
	
	fmt.Printf("===== RENDER DEBUG: window %d, frame size %d bytes =====\n", windowIndex, len(frameData))
	
	// Find the server monitor ID for this window index
	var monitorID uint32
	for sID, wIdx := range c.monitorMap {
		if wIdx == windowIndex {
			monitorID = sID
			break
		}
	}
	
	if monitorID == 0 {
		fmt.Printf("Warning: Unable to find server monitor ID for window %d\n", windowIndex)
		monitorID = uint32(windowIndex + 1) // Fallback
	}
	
	// Create debug frames directory
	debugDir := "debug_frames"
	if err := os.MkdirAll(debugDir, 0755); err != nil {
		fmt.Printf("Error creating debug directory: %v\n", err)
	}
	
	// Save raw JPEG data for manual inspection
	rawFrameFile := filepath.Join(debugDir, fmt.Sprintf("raw_frame_win%d_mon%d.jpg", windowIndex, monitorID))
	if err := os.WriteFile(rawFrameFile, frameData, 0644); err != nil {
		fmt.Printf("Error saving raw frame data: %v\n", err)
	} else {
		fmt.Printf("Saved raw JPEG data to %s\n", rawFrameFile)
	}
	
	// Check JPEG header
	if len(frameData) < 2 || frameData[0] != 0xFF || frameData[1] != 0xD8 {
		return fmt.Errorf("invalid JPEG header: first bytes: %x %x", frameData[0], frameData[1])
	}
	fmt.Println("JPEG header OK")
	
	// Decode JPEG data
	fmt.Println("Decoding JPEG into image...")
	img, err := jpeg.Decode(bytes.NewReader(frameData))
	if err != nil {
		fmt.Printf("JPEG decode error: %v\n", err)
		// Save frame to a file for inspection
		if fileErr := os.WriteFile("debug_frame.jpg", frameData, 0644); fileErr == nil {
			fmt.Println("Wrote debug frame to debug_frame.jpg")
		}
		return err
	}
	
	fmt.Printf("JPEG decoded successfully, size: %dx%d\n", img.Bounds().Dx(), img.Bounds().Dy())
	
	// Track frame count per monitor
	c.frameCount[monitorID]++
	
	// Save the decoded image to a file (both PNG and JPEG for comparison)
	jpgFilename := saveImageToFile(img, monitorID, c.frameCount[monitorID], "jpg")
	pngFilename := saveImageToFile(img, monitorID, c.frameCount[monitorID], "png")
	
	if jpgFilename != "" && pngFilename != "" {
		fmt.Printf("Saved decoded images to %s and %s\n", jpgFilename, pngFilename)
	}
	
	// Convert to RGBA
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	fmt.Printf("Image dimensions: %dx%d\n", bounds.Dx(), bounds.Dy())
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Over)
	fmt.Printf("Converted to RGBA, pixel buffer size: %d bytes\n", len(rgba.Pix))
	
	// Save the RGBA data as a PNG for inspection
	rgbaFilename := filepath.Join(debugDir, fmt.Sprintf("rgba_mon%d_%d.png", monitorID, c.frameCount[monitorID]))
	rgbaFile, err := os.Create(rgbaFilename)
	if err != nil {
		fmt.Printf("Error creating RGBA debug file: %v\n", err)
	} else {
		defer rgbaFile.Close()
		png.Encode(rgbaFile, rgba)
		fmt.Printf("Saved RGBA data to %s\n", rgbaFilename)
	}
	
	// Get or create the texture for this window
	texture, ok := c.textures[windowIndex]
	if !ok {
		texture = c.createTexture()
		fmt.Printf("Created new texture ID %d for window %d\n", texture, windowIndex)
		c.textures[windowIndex] = texture
	}
	
	// Debug OpenGL state
	window.MakeContextCurrent() // Make sure context is current
	var maxSize int32
	gl.GetIntegerv(gl.MAX_TEXTURE_SIZE, &maxSize)
	fmt.Printf("OpenGL MAX_TEXTURE_SIZE: %d\n", maxSize)
	
	// Update texture with RGBA data
	gl.BindTexture(gl.TEXTURE_2D, texture) 
	// Check errors after binding
	if glErr := gl.GetError(); glErr != gl.NO_ERROR {
		fmt.Printf("OpenGL error after texture bind: 0x%x\n", glErr)
		return fmt.Errorf("OpenGL error after bind: 0x%x", glErr)
	}
	
	// Force pixel storage alignment to 1 to handle any image size
	gl.PixelStorei(gl.UNPACK_ALIGNMENT, 1)
	gl.PixelStorei(gl.PACK_ALIGNMENT, 1)
	if pixelErr := gl.GetError(); pixelErr != gl.NO_ERROR {
		fmt.Printf("OpenGL error after setting pixel alignment: 0x%x\n", pixelErr)
	}
	
	// Upload texture data - carefully manage error checking
	gl.TexImage2D(
		gl.TEXTURE_2D,
		0,
		gl.RGBA,
		int32(bounds.Dx()),
		int32(bounds.Dy()),
		0,
		gl.RGBA,
		gl.UNSIGNED_BYTE,
		gl.Ptr(rgba.Pix))
		
	// Check for errors after texture upload
	if glErr := gl.GetError(); glErr != gl.NO_ERROR {
		fmt.Printf("OpenGL error after texture upload: 0x%x\n", glErr)
		return fmt.Errorf("failed to upload texture: 0x%x", glErr)
	} else if bounds.Dx() > 0 && bounds.Dy() > 0 {
		fmt.Printf("Texture upload successful for %dx%d image\n", bounds.Dx(), bounds.Dy())
	}
	return nil
}

// displayFrame displays a JPEG frame in the given window
func (c *SimpleClient) displayFrame(windowIndex int, frameData []byte) error {
	// Ensure we have the correct window context before anything else
	window := c.windows[windowIndex]
	if window == nil || window.ShouldClose() {
		return fmt.Errorf("window %d is nil or should close", windowIndex)
	}
	// Make window current to ensure proper OpenGL context
	window.MakeContextCurrent()

	// Render the frame to the window's texture
	err := c.renderFrame(windowIndex, frameData)
	if err != nil {
		return err
	}
	
	// Get the texture for this window
	texture, ok := c.textures[windowIndex]
	if !ok {
		return fmt.Errorf("no texture found for window %d", windowIndex)
	}
	
	// Print info for debugging
	fmt.Printf("Display frame: Window %d, TextureID: %d\n", windowIndex, texture)
	
	// Clear the window with a dark background
	gl.ClearColor(0.2, 0.2, 0.2, 1.0)
	gl.Clear(gl.COLOR_BUFFER_BIT)

	fmt.Printf("About to render texture with ID %d\n", texture)
	
	// SIMPLIFIED APPROACH - Only display the texture
	renderSimpleFullscreenTexture(texture)

	return nil
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
    
    // Draw a fullscreen quad with the texture - note correct orientation
    gl.Begin(gl.QUADS)
    
    // OpenGL has (0,0) at bottom-left, but image data (JPEG/PNG) has (0,0) at top-left
    // Flip Y-coordinates to fix the upside-down rendering - [0,0] at bottom-left
    // Flip Y-coordinates to match image data with origin at top-left
    gl.TexCoord2f(0.0, 0.0); gl.Vertex2f(0.0, 0.0) // Bottom-left
    gl.TexCoord2f(1.0, 0.0); gl.Vertex2f(1.0, 0.0) // Bottom-right
    gl.TexCoord2f(1.0, 0.0); gl.Vertex2f(1.0, 1.0) // Top-right
    gl.TexCoord2f(0.0, 0.0); gl.Vertex2f(0.0, 1.0) // Top-left
    
    gl.End()
    
    // Disable texturing when done
    gl.Disable(gl.TEXTURE_2D)
    
    // Check for errors
    if err := gl.GetError(); err != gl.NO_ERROR {
        fmt.Printf("OpenGL error in renderSimpleFullscreenTexture: 0x%x\n", err)
    } else {
        fmt.Println("Simple texture render completed successfully")
    }
}

// REMOVED all the old texture drawing functions to focus on a single approach

// networkHandler runs in a separate goroutine to handle network communication
func (c *SimpleClient) networkHandler() {
	fmt.Println("Starting network handler")
	
	// Perform handshake
	if err := c.handleHandshake(); err != nil {
		fmt.Printf("Handshake failed: %v\n", err)
		c.Stop()
		return
	}
	
	// Start packet receiver
	c.receivePackets()
}

// handleHandshake performs the initial handshake with the server
func (c *SimpleClient) handleHandshake() error {
	fmt.Println("Performing handshake with server...")
	fmt.Println("Waiting for server monitor configuration...")
	
	// Read handshake packet
	packet, err := protocol.DecodePacket(c.conn)
	if err != nil {
		return fmt.Errorf("failed to read handshake: %v", err)
	}
	
	if packet.Type != protocol.PacketTypeHandshake {
		return fmt.Errorf("unexpected packet type: %d", packet.Type)
	}
	
	// Decode server monitor configuration
	serverMonitors, err := protocol.DecodeMonitorConfig(packet.Payload)
	if err != nil {
		return fmt.Errorf("failed to decode server monitor config: %v", err)
	}
	
	c.serverMonitors = serverMonitors
	fmt.Printf("Server has %d monitors\n", serverMonitors.MonitorCount)
	
	// Send our monitor configuration
	monitorData := protocol.EncodeMonitorConfig(c.localMonitors)
	responsePacket := protocol.NewPacket(protocol.PacketTypeMonitorConfig, monitorData)
	
	if err := protocol.EncodePacket(c.conn, responsePacket); err != nil {
		return fmt.Errorf("failed to send monitor config: %v", err)
	}
	
	// Map server monitors to local monitors
	// For now, we use a simple 1:1 mapping
	for i := uint32(0); i < serverMonitors.MonitorCount && i < c.localMonitors.MonitorCount; i++ {
		serverID := serverMonitors.Monitors[i].ID
		localID := c.localMonitors.Monitors[i].ID
		
		// Store server monitor ID to window index mapping
		// Subtract 1 because our window indices are 0-based but monitor IDs are 1-based
		windowIndex := int(localID) - 1
		if windowIndex >= 0 && windowIndex < len(c.windows) {
			c.monitorMap[serverID] = windowIndex
			fmt.Printf("MAPPING: Server monitor %d -> Local monitor %d -> Window %d\n", 
				serverID, localID, windowIndex)
		}
	}
	
	fmt.Printf("Monitor mapping complete: %v\n", c.monitorMap)
	return nil
}

// receivePackets continuously receives packets from the server
func (c *SimpleClient) receivePackets() {
	fmt.Println("Starting packet receiver...")
	
	for !c.stopped {
		// Check if we should stop
		select {
		case <-c.stopChan:
			fmt.Println("Stopped packet receiver")
			return
		default:
			// Continue
		}
		
		// Set a read deadline to allow for checking the stop condition
		_ = c.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		
		// Try to read a packet
		packet, err := protocol.DecodePacket(c.conn)
		if err != nil {
			// Check if it's a timeout error
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			
			if !c.stopped {
				fmt.Printf("Error receiving packet: %v\n", err)
				c.Stop()
			}
			return
		}
		
		// Handle different packet types
		switch packet.Type {
		case protocol.PacketTypeVideoFrame:
			c.handleVideoFrame(packet.Payload)
		}
	}
}

// handleVideoFrame processes a video frame packet
func (c *SimpleClient) handleVideoFrame(payload []byte) {
	serverCount := 0
	
	if len(payload) < 4 {
		fmt.Println("Invalid video frame packet (too short)")
		return
	}
	
	// Extract monitor ID (first 4 bytes) and frame data (rest)
	monitorID := protocol.BytesToUint32(payload[0:4])
	frameData := payload[4:]
	
	// Check JPEG header
	if len(frameData) < 2 || frameData[0] != 0xFF || frameData[1] != 0xD8 {
		fmt.Printf("Invalid JPEG data for monitor %d\n", monitorID)
		return
	}
	
	// Get the window index for this monitor
	windowIndex, ok := c.monitorMap[monitorID]
	if !ok || windowIndex < 0 || windowIndex >= len(c.windows) {
		// Only log this once per server monitor ID
		if serverCount < 3 {
			fmt.Printf("WARNING: Received frame for unknown server monitor ID %d\n", monitorID)
			serverCount++
		}
		return
	} else {
		fmt.Printf("Frame for server monitor %d will render to window %d\n", monitorID, windowIndex)
	}
	
	// Update frame buffer
	c.frameMutex.Lock()
	// Copy to new buffer to avoid races
	if _, exists := c.frameBuffers[monitorID]; !exists {
		fmt.Printf("First frame received for monitor %d\n", monitorID)
	}
	c.frameBuffers[monitorID] = make([]byte, len(frameData)) 
	copy(c.frameBuffers[monitorID], frameData)
	c.frameMutex.Unlock()
	
	fmt.Printf("Received frame for monitor %d (%d bytes)\n", monitorID, len(frameData))
}