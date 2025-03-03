package server

import (
	"log"
	"github.com/kbinani/screenshot"
	"image"
	"image/jpeg"
	"bytes"
	"time"
	"github.com/moderniselife/ultrardp/protocol"
)

// startScreenCapture begins capturing and encoding screen content
func (s *Server) startScreenCapture() {
	// Create a capture routine for each monitor
	for _, monitor := range s.monitors.Monitors {
		go s.captureMonitor(monitor)
	}
}

// captureMonitor captures and encodes frames from a single monitor
func (s *Server) captureMonitor(monitor protocol.MonitorInfo) {
	log.Printf("Started capture for monitor %d (%dx%d)", 
		monitor.ID, monitor.Width, monitor.Height)

	// Create a buffer for JPEG encoding
	buf := new(bytes.Buffer)

	for !s.stopped {
		// Capture the screen
		bound := image.Rect(int(monitor.PositionX), int(monitor.PositionY),
			int(monitor.PositionX)+int(monitor.Width), int(monitor.PositionY)+int(monitor.Height))
		img, err := screenshot.CaptureRect(bound)
		if err != nil {
			log.Printf("Error capturing screen: %v", err)
			continue
		}

		// Reset buffer
		buf.Reset()

		// Encode as JPEG (can be replaced with hardware H.264 encoding later)
		if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: 80}); err != nil {
			log.Printf("Error encoding frame: %v", err)
			continue
		}

		// Prepare frame packet
		frameData := make([]byte, 4+buf.Len())
		// Add monitor ID
		copy(frameData[0:4], protocol.Uint32ToBytes(monitor.ID))
		// Add frame data
		copy(frameData[4:], buf.Bytes())

		// Send to all connected clients
		s.clientsMutex.Lock()
		for _, client := range s.clients {
			if !client.active {
				continue
			}
			
			// Check if this monitor is mapped for this client
			if _, ok := client.monitorMap[monitor.ID]; !ok {
				continue
			}

			// Send frame packet
			packet := protocol.NewPacket(protocol.PacketTypeVideoFrame, frameData)
			if err := protocol.EncodePacket(client.conn, packet); err != nil {
				log.Printf("Error sending frame to client %s: %v", client.id, err)
				client.active = false
			}
		}
		s.clientsMutex.Unlock()

		// Sleep to maintain target frame rate (60fps)
		time.Sleep(16 * time.Millisecond)
	}
}