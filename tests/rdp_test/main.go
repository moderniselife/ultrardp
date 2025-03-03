package main

import (
	"fmt"
	"log"
	"net"
	"runtime"
	"sync"
	"time"

	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/moderniselife/ultrardp/protocol"
)

type SimpleClient struct {
	conn           net.Conn
	serverMonitors *protocol.MonitorConfig
	localMonitors  *protocol.MonitorConfig
	stopped        bool
	frameMutex     sync.Mutex
	frameBuffers   map[uint32][]byte
	windows        []*glfw.Window
}

func main() {
	fmt.Println("Starting simplified RDP client")
	
	// GLFW operations must run on the main thread
	runtime.LockOSThread()
	
	// Create client
	client, err := NewSimpleClient("100.124.193.59:8000")
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	
	// Execute handshake
	if err := client.handleHandshake(); err != nil {
		log.Fatalf("Handshake failed: %v", err)
	}

	// Start receiving packets in a goroutine
	go client.receivePackets()
	
	// The display loop runs on the main thread
	client.displayLoop()
}

func NewSimpleClient(address string) (*SimpleClient, error) {
	fmt.Println("Connecting to server...")
	
	// Connect to server
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %v", err)
	}
	
	// Detect local monitors - hard-coded for simplicity
	monitors := &protocol.MonitorConfig{
		MonitorCount: 1,
		Monitors: []protocol.MonitorInfo{
			{
				ID:        1,
				Width:     1920,
				Height:    1080,
				PositionX: 0,
				PositionY: 0,
				Primary:   true,
			},
		},
	}
	
	client := &SimpleClient{
		conn:          conn,
		localMonitors: monitors,
		frameBuffers:  make(map[uint32][]byte),
		stopped:       false,
	}
	
	return client, nil
}

func (c *SimpleClient) handleHandshake() error {
	fmt.Println("Performing handshake...")
	
	// Receive server's monitor configuration
	packet, err := protocol.DecodePacket(c.conn)
	if err != nil {
		return fmt.Errorf("failed to decode packet: %v", err)
	}
	
	if packet.Type != protocol.PacketTypeHandshake {
		return fmt.Errorf("expected handshake packet, got %d", packet.Type)
	}
	
	// Decode server monitor configuration
	serverMonitors, err := protocol.DecodeMonitorConfig(packet.Payload)
	if err != nil {
		return fmt.Errorf("failed to decode monitor config: %v", err)
	}
	
	c.serverMonitors = serverMonitors
	fmt.Printf("Server has %d monitors\n", serverMonitors.MonitorCount)
	
	// Send our monitor configuration to the server
	monitorData := protocol.EncodeMonitorConfig(c.localMonitors)
	responsePacket := protocol.NewPacket(protocol.PacketTypeMonitorConfig, monitorData)
	
	if err := protocol.EncodePacket(c.conn, responsePacket); err != nil {
		return fmt.Errorf("failed to send monitor config: %v", err)
	}
	
	// Map server monitors to local monitors
	for i := uint32(0); i < serverMonitors.MonitorCount && i < c.localMonitors.MonitorCount; i++ {
		serverMonitor := serverMonitors.Monitors[i]
		localMonitor := c.localMonitors.Monitors[i]
		fmt.Printf("Mapped server monitor %d to local monitor %d\n", serverMonitor.ID, localMonitor.ID)
	}
	
	return nil
}

func (c *SimpleClient) receivePackets() {
	fmt.Println("Starting packet receiver...")
	
	for !c.stopped {
		packet, err := protocol.DecodePacket(c.conn)
		if err != nil {
			if !c.stopped {
				fmt.Printf("Error receiving packet: %v\n", err)
			}
			break
		}
		
		// Handle the packet
		switch packet.Type {
		case protocol.PacketTypeVideoFrame:
			// Process video frame
			if len(packet.Payload) < 4 {
				fmt.Println("Invalid video frame packet")
				continue
			}
			
			// Extract monitor ID and frame data
			monitorID := protocol.BytesToUint32(packet.Payload[0:4])
			frameData := packet.Payload[4:]
			
			// Update frame buffer
			c.frameMutex.Lock()
			c.frameBuffers[monitorID] = frameData
			c.frameMutex.Unlock()
			
			fmt.Printf("Received frame for monitor %d (%d bytes)\n", monitorID, len(frameData))
		}
	}
}

func (c *SimpleClient) displayLoop() {
	fmt.Println("Starting display loop...")
	
	// Initialize GLFW
	err := glfw.Init()
	if err != nil {
		fmt.Printf("Failed to initialize GLFW: %v\n", err)
		return
	}
	defer glfw.Terminate()
	
	fmt.Printf("GLFW initialized successfully, version: %s\n", glfw.GetVersionString())
	
	// Print monitor info
	monitors := glfw.GetMonitors()
	fmt.Printf("Found %d monitors\n", len(monitors))
	for i, monitor := range monitors {
		x, y := monitor.GetPos()
		w, h := monitor.GetVideoMode().Width, monitor.GetVideoMode().Height
		fmt.Printf("Monitor %d: %s at (%d,%d) resolution %dx%d\n", 
			i, monitor.GetName(), x, y, w, h)
	}
	
	// Set window hints
	glfw.DefaultWindowHints()
	glfw.WindowHint(glfw.Visible, glfw.True)
	glfw.WindowHint(glfw.Decorated, glfw.True)
	glfw.WindowHint(glfw.Resizable, glfw.False)
	glfw.WindowHint(glfw.Focused, glfw.True)
	
	// Create window for each monitor (just one for simplicity)
	window, err := glfw.CreateWindow(640, 480, "UltraRDP Test", nil, nil)
	if err != nil {
		fmt.Printf("Failed to create window: %v\n", err)
		return
	}
	defer window.Destroy()
	
	// Position and show window
	window.SetPos(100, 100)
	window.Show()
	
	c.windows = []*glfw.Window{window}
	fmt.Println("Window created successfully!")
	
	// Main display loop
	for !c.stopped && !window.ShouldClose() {
		// Poll for events
		glfw.PollEvents()
		
		// Keep updating the display for at least 30 seconds
		time.Sleep(100 * time.Millisecond)
	}
	
	fmt.Println("Display loop ended")
}