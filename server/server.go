package server

import (
	"fmt"
	"log"
	"net"
	"sync"
	"github.com/kbinani/screenshot"
	"github.com/moderniselife/ultrardp/protocol"
)

// Server represents an UltraRDP server instance
type Server struct {
	address      string
	listener     net.Listener
	clients      map[string]*Client
	clientsMutex sync.Mutex
	monitors     *protocol.MonitorConfig
	stopped      bool
}

// Client represents a connected client
type Client struct {
	id         string
	conn       net.Conn
	active     bool
	monitorMap map[uint32]uint32
	monitors   *protocol.MonitorConfig
}

// NewServer creates a new UltraRDP server
func NewServer(address string) (*Server, error) {
	// Detect monitors
	monitors, err := detectMonitors()
	if err != nil {
		return nil, err
	}

	return &Server{
		address:  address,
		clients:  make(map[string]*Client),
		monitors: monitors,
		stopped:  false,
	}, nil
}

// Start begins the server's main loop
func (s *Server) Start() error {
	// Create TCP listener
	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		return err
	}
	s.listener = listener

	// Start screen capture
	s.startScreenCapture()

	// Accept client connections
	for !s.stopped {
		conn, err := listener.Accept()
		if err != nil {
			if s.stopped {
				break
			}
			log.Printf("Error accepting connection: %v", err)
			continue
		}

		// Handle client in a goroutine
		go s.handleClient(conn)
	}

	return nil
}

// Stop shuts down the server
func (s *Server) Stop() {
	s.stopped = true
	if s.listener != nil {
		s.listener.Close()
	}

	// Close all client connections
	s.clientsMutex.Lock()
	for _, client := range s.clients {
		client.conn.Close()
	}
	s.clientsMutex.Unlock()
}

// handleClient processes a client connection
func (s *Server) handleClient(conn net.Conn) {
	// Send our monitor configuration to the client
	monitorData := protocol.EncodeMonitorConfig(s.monitors)
	handshakePacket := protocol.NewPacket(protocol.PacketTypeHandshake, monitorData)
	
	if err := protocol.EncodePacket(conn, handshakePacket); err != nil {
		log.Printf("Failed to send handshake packet: %v", err)
		conn.Close()
		return
	}
	
	// Receive client's monitor configuration
	packet, err := protocol.DecodePacket(conn)
	if err != nil {
		log.Printf("Failed to receive client monitor config: %v", err)
		conn.Close()
		return
	}
	
	if packet.Type != protocol.PacketTypeMonitorConfig {
		log.Printf("Expected monitor config packet, got %d", packet.Type)
		conn.Close()
		return
	}
	
	// Decode client monitor configuration
	clientMonitors, err := protocol.DecodeMonitorConfig(packet.Payload)
	if err != nil {
		log.Printf("Failed to decode client monitor config: %v", err)
		conn.Close()
		return
	}
	
	// Create new client instance
	client := &Client{
		conn:       conn,
		monitors:   clientMonitors,
		active:     true,
		id:         conn.RemoteAddr().String(),
		monitorMap: make(map[uint32]uint32),
	}
	
	// Create monitor mapping
	for i := uint32(0); i < s.monitors.MonitorCount && i < clientMonitors.MonitorCount; i++ {
		serverMonitor := s.monitors.Monitors[i]
		clientMonitor := clientMonitors.Monitors[i]
		client.monitorMap[serverMonitor.ID] = clientMonitor.ID
		log.Printf("Mapped server monitor %d to client monitor %d", serverMonitor.ID, clientMonitor.ID)
	}
	
	// Add client to server's client list
	s.clientsMutex.Lock()
	s.clients[conn.RemoteAddr().String()] = client
	s.clientsMutex.Unlock()
	
	log.Printf("Client connected from %s with %d monitors", conn.RemoteAddr(), clientMonitors.MonitorCount)
	
	// TODO: Start handling client communication (streaming, input, etc.)
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