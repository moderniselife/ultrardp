// Package server implements the UltraRDP server functionality
package server

import (
	"fmt"
	"log"
	"net"
	"sync"
	
	"github.com/moderniselife/ultrardp/protocol"
)

// Server represents an UltraRDP server instance
type Server struct {
	listener     net.Listener
	clients      map[string]*Client
	clientsMutex sync.Mutex
	monitors     *protocol.MonitorConfig
	stopped      bool
	stopChan     chan struct{}
}

// Client represents a connected client
type Client struct {
	conn         net.Conn
	id           string
	monitorMap   map[uint32]uint32 // Maps server monitor IDs to client monitor IDs
	qualityLevel int               // 0-100, where 100 is highest quality
	active       bool
}

// NewServer creates a new UltraRDP server
func NewServer(address string) (*Server, error) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to start server: %w", err)
	}

	// Detect monitors
	monitors, err := detectMonitors()
	if err != nil {
		return nil, fmt.Errorf("failed to detect monitors: %w", err)
	}

	return &Server{
		listener:     listener,
		clients:      make(map[string]*Client),
		clientsMutex: sync.Mutex{},
		monitors:     monitors,
		stopped:      false,
		stopChan:     make(chan struct{}),
	}, nil
}

// Start begins accepting client connections
func (s *Server) Start() error {
	log.Println("Server started, detected", s.monitors.MonitorCount, "monitors")
	
	// Start screen capture for all monitors
	go s.startScreenCapture()
	
	for !s.stopped {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.stopped {
				break
			}
			log.Println("Error accepting connection:", err)
			continue
		}
		
		go s.handleClient(conn)
	}
	
	return nil
}

// Stop shuts down the server
func (s *Server) Stop() {
	s.stopped = true
	close(s.stopChan)
	s.listener.Close()
	
	// Close all client connections
	s.clientsMutex.Lock()
	defer s.clientsMutex.Unlock()
	
	for _, client := range s.clients {
		client.conn.Close()
	}
}

// handleClient processes a new client connection
func (s *Server) handleClient(conn net.Conn) {
	clientID := conn.RemoteAddr().String()
	log.Println("New client connected:", clientID)
	
	client := &Client{
		conn:         conn,
		id:           clientID,
		monitorMap:   make(map[uint32]uint32),
		qualityLevel: 80, // Default quality level
		active:       true,
	}
	
	// Add client to clients map
	s.clientsMutex.Lock()
	s.clients[clientID] = client
	s.clientsMutex.Unlock()
	
	defer func() {
		conn.Close()
		s.clientsMutex.Lock()
		delete(s.clients, clientID)
		s.clientsMutex.Unlock()
		log.Println("Client disconnected:", clientID)
	}()
	
	// Send initial handshake with monitor configuration
	if err := s.sendHandshake(client); err != nil {
		log.Println("Error sending handshake:", err)
		return
	}
	
	// Handle client packets
	for client.active {
		packet, err := protocol.DecodePacket(conn)
		if err != nil {
			log.Println("Error reading packet:", err)
			break
		}
		
		s.handlePacket(client, packet)
	}
}

// sendHandshake sends the initial handshake to a client
func (s *Server) sendHandshake(client *Client) error {
	// Encode monitor configuration
	monitorData := protocol.EncodeMonitorConfig(s.monitors)
	
	// Create handshake packet
	packet := protocol.NewPacket(protocol.PacketTypeHandshake, monitorData)
	
	// Send packet
	return protocol.EncodePacket(client.conn, packet)
}

// handlePacket processes an incoming packet from a client
func (s *Server) handlePacket(client *Client, packet *protocol.Packet) {
	switch packet.Type {
	case protocol.PacketTypeMonitorConfig:
		// Client is sending its monitor configuration
		clientMonitors, err := protocol.DecodeMonitorConfig(packet.Payload)
		if err != nil {
			log.Println("Error decoding client monitor config:", err)
			return
		}
		
		// Map client monitors to server monitors
		s.mapMonitors(client, clientMonitors)
		
	case protocol.PacketTypeMouseMove:
		// Handle mouse movement
		// TODO: Implement input handling
		
	case protocol.PacketTypeMouseButton:
		// Handle mouse button
		// TODO: Implement input handling
		
	case protocol.PacketTypeKeyboard:
		// Handle keyboard input
		// TODO: Implement input handling
		
	case protocol.PacketTypeQualityControl:
		// Client is requesting quality adjustment
		if len(packet.Payload) >= 1 {
			client.qualityLevel = int(packet.Payload[0])
			log.Printf("Client %s quality set to %d", client.id, client.qualityLevel)
		}
		
	case protocol.PacketTypePing:
		// Respond with pong
		pongPacket := protocol.NewPacket(protocol.PacketTypePong, packet.Payload)
		protocol.EncodePacket(client.conn, pongPacket)
	}
}

// mapMonitors creates a mapping between server and client monitors
func (s *Server) mapMonitors(client *Client, clientMonitors *protocol.MonitorConfig) {
	// Clear existing mapping
	client.monitorMap = make(map[uint32]uint32)
	
	// Simple 1:1 mapping for now
	// In a real implementation, this would be more sophisticated based on
	// monitor resolutions, positions, etc.
	for i := uint32(0); i < s.monitors.MonitorCount && i < clientMonitors.MonitorCount; i++ {
		serverMonitor := s.monitors.Monitors[i]
		clientMonitor := clientMonitors.Monitors[i]
		
		client.monitorMap[serverMonitor.ID] = clientMonitor.ID
		log.Printf("Mapped server monitor %d to client monitor %d", 
			serverMonitor.ID, clientMonitor.ID)
	}
}

// startScreenCapture begins capturing and encoding screen content
func (s *Server) startScreenCapture() {
	// Create a capture routine for each monitor
	for _, monitor := range s.monitors.Monitors {
		go s.captureMonitor(monitor)
	}
}

// captureMonitor captures and encodes frames from a single monitor
func (s *Server) captureMonitor(monitor protocol.MonitorInfo) {
	// TODO: Implement screen capture using platform-specific APIs
	// This would use libraries like:
	// - Windows: Desktop Duplication API
	// - macOS: AVFoundation or CGDisplayStream
	// - Linux: X11 or Wayland APIs
	
	log.Printf("Started capture for monitor %d (%dx%d)", 
		monitor.ID, monitor.Width, monitor.Height)
	
	// Placeholder for capture loop
	for !s.stopped {
		// 1. Capture frame
		// 2. Encode frame (H.264/HEVC using hardware acceleration)
		// 3. Send to all clients that have this monitor mapped
		
		// Sleep to simulate frame capture at target rate
		// For 240fps, sleep for approximately 4ms
		// time.Sleep(4 * time.Millisecond)
		
		// Check if we should stop
		select {
		case <-s.stopChan:
			return
		default:
			// Continue capturing
		}
	}
}

// detectMonitors identifies the available monitors on the system
func detectMonitors() (*protocol.MonitorConfig, error) {
	// TODO: Implement platform-specific monitor detection
	// This is a placeholder implementation
	
	// Create a dummy monitor configuration for testing
	config := &protocol.MonitorConfig{
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
	
	return config, nil
}