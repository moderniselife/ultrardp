// Package client implements the UltraRDP client functionality
package client

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"log"
	"net"
	"sync"
	"runtime"
	
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/kbinani/screenshot"
	"github.com/moderniselife/ultrardp/protocol"
)

// Client represents an UltraRDP client instance
type Client struct {
	conn           net.Conn
	serverMonitors *protocol.MonitorConfig
	localMonitors  *protocol.MonitorConfig
	monitorMap     map[uint32]uint32 // Maps server monitor IDs to local monitor IDs
	qualityLevel   int               // 0-100, where 100 is highest quality
	stopped        bool
	stopChan       chan struct{}
	frameMutex     sync.Mutex
	frameBuffers   map[uint32][]byte // Buffers for each monitor
	windows        []*glfw.Window    // Windows for displaying frames
}

// NewClient creates a new UltraRDP client
func NewClient(address string) (*Client, error) {
	// Detect local monitors
	localMonitors, err := detectMonitors()
	if err != nil {
		return nil, fmt.Errorf("failed to detect local monitors: %w", err)
	}
	
	// Connect to server
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}
	
	return &Client{
		conn:           conn,
		localMonitors:  localMonitors,
		monitorMap:     make(map[uint32]uint32),
		qualityLevel:   80, // Default quality level
		stopped:        false,
		stopChan:       make(chan struct{}),
		frameBuffers:   make(map[uint32][]byte),
	}, nil
}

// Start begins the client session
func (c *Client) Start() error {
	log.Println("Client started, detected", c.localMonitors.MonitorCount, "local monitors")
	
	// Handle initial handshake
	if err := c.handleHandshake(); err != nil {
		return fmt.Errorf("handshake failed: %w", err)
	}
	
	// Start input capture in a goroutine
	go c.startInputCapture()
	
	// Start packet receiving loop in a goroutine
	go func() {
		for !c.stopped {
			packet, err := protocol.DecodePacket(c.conn)
			if err != nil {
				if !c.stopped {
					log.Printf("Error receiving packet: %v", err)
				}
				break
			}
			c.handlePacket(packet)
		}
	}()
	
	// Start display loop on main thread
	c.startDisplayLoop()
	
	return nil
}

// Stop shuts down the client
func (c *Client) Stop() {
	c.stopped = true
	close(c.stopChan)
	if c.conn != nil {
		c.conn.Close()
	}
}

// handleHandshake processes the initial handshake with the server
func (c *Client) handleHandshake() error {
	// Receive server's monitor configuration
	packet, err := protocol.DecodePacket(c.conn)
	if err != nil {
		return err
	}
	
	if packet.Type != protocol.PacketTypeHandshake {
		return fmt.Errorf("expected handshake packet, got %d", packet.Type)
	}
	
	// Decode server monitor configuration
	serverMonitors, err := protocol.DecodeMonitorConfig(packet.Payload)
	if err != nil {
		return err
	}
	
	c.serverMonitors = serverMonitors
	log.Printf("Server has %d monitors", serverMonitors.MonitorCount)
	
	// Send our monitor configuration to the server
	monitorData := protocol.EncodeMonitorConfig(c.localMonitors)
	responsePacket := protocol.NewPacket(protocol.PacketTypeMonitorConfig, monitorData)
	
	if err := protocol.EncodePacket(c.conn, responsePacket); err != nil {
		return err
	}
	
	// Create monitor mapping
	c.createMonitorMapping()
	
	return nil
}

// createMonitorMapping maps server monitors to local monitors
func (c *Client) createMonitorMapping() {
	// Clear existing mapping
	c.monitorMap = make(map[uint32]uint32)
	
	// Simple 1:1 mapping for now
	// In a real implementation, this would be more sophisticated based on
	// monitor resolutions, positions, etc.
	for i := uint32(0); i < c.serverMonitors.MonitorCount && i < c.localMonitors.MonitorCount; i++ {
		serverMonitor := c.serverMonitors.Monitors[i]
		localMonitor := c.localMonitors.Monitors[i]
		
		c.monitorMap[serverMonitor.ID] = localMonitor.ID
		log.Printf("Mapped server monitor %d to local monitor %d", 
			serverMonitor.ID, localMonitor.ID)
		
		// Initialize frame buffer for this monitor with a reasonable initial size
		c.frameBuffers[localMonitor.ID] = make([]byte, int(localMonitor.Width*localMonitor.Height*4)) // 4 bytes per pixel (RGBA)
	}
}

// handlePacket processes an incoming packet from the server
func (c *Client) handlePacket(packet *protocol.Packet) {
    switch packet.Type {
    case protocol.PacketTypeVideoFrame:
        // Process video frame
        if len(packet.Payload) < 4 {
            log.Println("Invalid video frame packet")
            return
        }
        
        // First 4 bytes contain the monitor ID
        monitorID := protocol.BytesToUint32(packet.Payload[0:4])
        frameData := packet.Payload[4:]
        
        // Update frame buffer for this monitor
        c.updateFrameBuffer(monitorID, frameData)
        
    case protocol.PacketTypeAudioFrame:
        // Process audio frame
        // TODO: Implement audio playback
        
    case protocol.PacketTypePong:
        // Process pong response (for latency measurement)
        // TODO: Calculate and display latency
        
    case protocol.PacketTypeMonitorConfig:
        // Server is sending an updated monitor configuration
        serverMonitors, err := protocol.DecodeMonitorConfig(packet.Payload)
        if err != nil {
            log.Println("Error decoding server monitor config:", err)
            return
        }
        
        c.serverMonitors = serverMonitors
        c.createMonitorMapping()
    }
}

// updateFrameBuffer updates the frame buffer for a specific monitor
func (c *Client) updateFrameBuffer(serverMonitorID uint32, frameData []byte) {
    c.frameMutex.Lock()
    defer c.frameMutex.Unlock()
    
    // Map server monitor ID to local monitor ID
    localMonitorID, ok := c.monitorMap[serverMonitorID]
    if !ok {
        log.Printf("No mapping found for server monitor ID %d", serverMonitorID)
        return
    }
    
    // Decode JPEG frame data
    img, err := jpeg.Decode(bytes.NewReader(frameData))
    if err != nil {
        log.Printf("Error decoding JPEG frame: %v", err)
        return
    }

    // Convert to RGBA
    bounds := img.Bounds()
    rgba := image.NewRGBA(bounds)
    draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)

    // Store the RGBA pixel data
    c.frameBuffers[localMonitorID] = rgba.Pix
    
    log.Printf("Updated frame buffer for monitor %d with %d bytes of RGBA data", localMonitorID, len(rgba.Pix))
}

// startDisplayLoop begins the display loop for rendering frames
func (c *Client) startDisplayLoop() {
	// Initialize GLFW in the main thread
	runtime.LockOSThread()

	// Call the display loop
	c.updateDisplayLoop()
}

// startInputCapture begins capturing user input
func (c *Client) startInputCapture() {
	// TODO: Implement platform-specific input capture
	// This would use libraries like:
	// - Windows: Raw Input API
	// - macOS: Quartz Event Services
	// - Linux: X11 or Wayland input APIs
	
	log.Println("Input capture started")
	
	// Placeholder for input capture loop
	for !c.stopped {
		// 1. Capture mouse/keyboard events
		// 2. Create appropriate packets
		// 3. Send to server
		
		// Check if we should stop
		select {
		case <-c.stopChan:
			return
		default:
			// Continue capturing
		}
	}
}

// SendQualityControl sends a quality control packet to the server
func (c *Client) SendQualityControl(quality int) error {
	if quality < 0 {
		quality = 0
	} else if quality > 100 {
		quality = 100
	}
	
	c.qualityLevel = quality
	
	// Create quality control packet
	payload := []byte{byte(quality)}
	packet := protocol.NewPacket(protocol.PacketTypeQualityControl, payload)
	
	return protocol.EncodePacket(c.conn, packet)
}

// SendPing sends a ping packet to measure latency
func (c *Client) SendPing() error {
	// Create ping packet with current timestamp
	packet := protocol.NewPacket(protocol.PacketTypePing, nil)
	
	return protocol.EncodePacket(c.conn, packet)
}

// detectMonitors identifies the available monitors on the system
func detectMonitors() (*protocol.MonitorConfig, error) {
	// Get all active displays using screenshot package
	displays := screenshot.NumActiveDisplays()
	if displays < 1 {
		return nil, fmt.Errorf("no active displays found")
	}

	// Create monitor config
	config := &protocol.MonitorConfig{
		MonitorCount: uint32(displays),
		Monitors:     make([]protocol.MonitorInfo, displays),
	}

	// Get information for each display
	for i := 0; i < displays; i++ {
		bounds := screenshot.GetDisplayBounds(i)
		config.Monitors[i] = protocol.MonitorInfo{
			ID:        uint32(i + 1),
			Width:     uint32(bounds.Dx()),
			Height:    uint32(bounds.Dy()),
			PositionX: uint32(bounds.Min.X),
			PositionY: uint32(bounds.Min.Y),
			Primary:   i == 0, // Assume first display is primary
		}
	}

	return config, nil
}