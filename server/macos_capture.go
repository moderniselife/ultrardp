package server

import (
	"log"
	"github.com/kbinani/screenshot"
	"image"
	"image/jpeg"
	"image/png"
	"bytes"
	"os"
	"path/filepath"
	"fmt"
	"time"
	"github.com/moderniselife/ultrardp/protocol"
)

// startScreenCapture begins capturing and encoding screen content
func (s *Server) startScreenCapture() {
	// Create debug directory
	debugDir := "debug_captures"
	if err := os.MkdirAll(debugDir, 0755); err != nil {
		log.Printf("Warning: Could not create debug directory: %v", err)
	}

	// Create a capture routine for each monitor
	for _, monitor := range s.monitors.Monitors {
		go s.captureMonitor(monitor)
	}
}

// captureMonitor captures and encodes frames from a single monitor
func (s *Server) captureMonitor(monitor protocol.MonitorInfo) {
	log.Printf("Started capture for monitor %d (%dx%d) at position (%d,%d)", 
		monitor.ID, monitor.Width, monitor.Height, monitor.PositionX, monitor.PositionY)

	// Create a buffer for JPEG encoding
	buf := new(bytes.Buffer)
	
	// Debug directory
	debugDir := "debug_captures"
	
	// Capture frame counter for this monitor
	frameCount := 0

	// Check if monitor coordinates look valid
	isValidCoords := true
	if monitor.PositionX > 10000 || monitor.PositionY > 10000 {
		log.Printf("WARNING: Invalid monitor coordinates detected for monitor %d: (%d,%d)",
			monitor.ID, monitor.PositionX, monitor.PositionY)
		isValidCoords = false
	}

	// Give server time to initialize and accept client connections
	time.Sleep(1 * time.Second)

	framesSent := 0
	lastClientCountLog := time.Now()

	for !s.stopped {
		var img image.Image
		var err error
		
		// Wait for at least one client to connect before starting to capture
		s.clientsMutex.Lock()
		clientCount := len(s.clients)
		s.clientsMutex.Unlock()
		
		if clientCount == 0 {
			if time.Since(lastClientCountLog) > 5*time.Second {
				log.Printf("No clients connected, waiting for connection before capturing monitor %d...", 
					monitor.ID)
				lastClientCountLog = time.Now()
			}
			time.Sleep(500 * time.Millisecond)
			continue
		}
		
		// Log client count occasionally
		if time.Since(lastClientCountLog) > 10*time.Second {
			log.Printf("Currently serving %d clients for monitor %d", clientCount, monitor.ID)
			lastClientCountLog = time.Now()
		}
		
		// Use different capture methods based on the monitor
		displayIndex := int(monitor.ID) - 1 // Convert 1-based ID to 0-based index
		
		if isValidCoords {
			// Try with coordinates first if they seem valid
			bound := image.Rect(int(monitor.PositionX), int(monitor.PositionY),
				int(monitor.PositionX)+int(monitor.Width), int(monitor.PositionY)+int(monitor.Height))
			
			if frameCount % 30 == 0 {
				log.Printf("Capturing monitor %d with bounds: %v", monitor.ID, bound)
			}
			img, err = screenshot.CaptureRect(bound)
		} else {
			// For monitors with suspect coordinates, use display index directly
			if displayIndex >= 0 && displayIndex < screenshot.NumActiveDisplays() {
				if frameCount % 30 == 0 {
					log.Printf("Capturing monitor %d using display index %d", monitor.ID, displayIndex)
				}
				img, err = screenshot.CaptureDisplay(displayIndex)
			} else {
				log.Printf("Invalid display index %d (num displays: %d)", 
					displayIndex, screenshot.NumActiveDisplays())
				time.Sleep(1 * time.Second)
				continue
			}
		}
		
		if err != nil {
			log.Printf("Error capturing screen: %v", err)
			
			// Try fallback if primary method fails
			if isValidCoords && displayIndex >= 0 && displayIndex < screenshot.NumActiveDisplays() {
				log.Printf("Trying fallback capture for display %d", displayIndex)
				img, err = screenshot.CaptureDisplay(displayIndex)
				if err != nil {
					log.Printf("Fallback capture also failed: %v", err)
					time.Sleep(1 * time.Second) // Wait longer after error
					continue
				}
			} else {
				time.Sleep(1 * time.Second)
				continue
			}
		}
		
		// Save a debug capture occasionally
		frameCount++
		if frameCount % 30 == 0 {
			debugPath := filepath.Join(debugDir, fmt.Sprintf("capture_mon%d_%d.png", monitor.ID, frameCount))
			debugFile, err := os.Create(debugPath)
			if err == nil {
				png.Encode(debugFile, img)
				debugFile.Close()
				log.Printf("Saved debug capture to %s", debugPath)
			}
		}

		// Check if the image is valid and not empty
		bounds := img.Bounds()
		if bounds.Empty() {
			log.Printf("Warning: Empty image captured for monitor %d", monitor.ID)
			time.Sleep(100 * time.Millisecond)
			continue
		}
		
		// Verify image isn't all black
		isBlack := true
		for y := bounds.Min.Y; y < bounds.Max.Y; y += bounds.Dy() / 10 {
			for x := bounds.Min.X; x < bounds.Max.X; x += bounds.Dx() / 10 {
				r, g, b, _ := img.At(x, y).RGBA()
				if r > 0 || g > 0 || b > 0 {
					isBlack = false
					break
				}
			}
			if !isBlack {
				break
			}
		}
		
		if isBlack {
			log.Printf("Warning: Black image captured for monitor %d", monitor.ID)
			// Try the direct method if we're still getting black images
			if isValidCoords && frameCount % 10 == 0 {
				log.Printf("Trying alternative capture method for monitor %d", monitor.ID)
				if displayIndex >= 0 && displayIndex < screenshot.NumActiveDisplays() {
					altImg, altErr := screenshot.CaptureDisplay(displayIndex)
					if altErr == nil {
						img = altImg
						// Check if the alternative image is also black
						isAltBlack := true
						for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y += img.Bounds().Dy() / 10 {
							for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x += img.Bounds().Dx() / 10 {
								r, g, b, _ := img.At(x, y).RGBA()
								if r > 0 || g > 0 || b > 0 {
									isAltBlack = false
									break
								}
							}
							if !isAltBlack {
								break
							}
						}
						if isAltBlack {
							log.Printf("Alternative method also produced black image for monitor %d", monitor.ID)
						} else {
							log.Printf("Alternative method succeeded for monitor %d", monitor.ID)
							isBlack = false
						}
					}
				}
			}
			
			// Save black images for debugging
			if frameCount % 5 == 0 {
				blackDebugPath := filepath.Join(debugDir, fmt.Sprintf("black_mon%d_%d.png", monitor.ID, frameCount))
				blackDebugFile, err := os.Create(blackDebugPath)
				if err == nil {
					png.Encode(blackDebugFile, img)
					blackDebugFile.Close()
					log.Printf("Saved black capture to %s", blackDebugPath)
				}
			}
		}

		// Reset buffer
		buf.Reset()

		// Encode as JPEG with higher quality for better visibility
		if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: 90}); err != nil {
			log.Printf("Error encoding frame: %v", err)
			continue
		}
		
		// Save JPEG occasionally to verify encoding
		if frameCount % 30 == 0 {
			jpegPath := filepath.Join(debugDir, fmt.Sprintf("encoded_mon%d_%d.jpg", monitor.ID, frameCount))
			if err := os.WriteFile(jpegPath, buf.Bytes(), 0644); err == nil {
				log.Printf("Saved encoded JPEG to %s", jpegPath)
			}
		}

		// Prepare frame packet
		frameData := make([]byte, 4+buf.Len())
		// Add monitor ID
		copy(frameData[0:4], protocol.Uint32ToBytes(monitor.ID))
		// Add frame data
		copy(frameData[4:], buf.Bytes())

		// Track clients that received the frame
		clientsReceived := 0

		// Send to all connected clients
		s.clientsMutex.Lock()
		for _, client := range s.clients {
			if !client.active {
				continue
			}
			
			// Check if this monitor is mapped for this client
			clientMonitorID, ok := client.monitorMap[monitor.ID]
			if !ok {
				// This is a common case if monitor counts don't match, so no need to log every time
				continue
			}

			// Log monitor mapping occasionally
			if frameCount % 30 == 0 {
				log.Printf("Sending frame %d for server monitor %d to client %s (mapped to client monitor %d)",
					frameCount, monitor.ID, client.id, clientMonitorID)
			}

			// Send frame packet
			packet := protocol.NewPacket(protocol.PacketTypeVideoFrame, frameData)
			if err := protocol.EncodePacket(client.conn, packet); err != nil {
				log.Printf("Error sending frame to client %s: %v", client.id, err)
				client.active = false
			} else {
				clientsReceived++
				
				if frameCount % 30 == 0 {
					log.Printf("Successfully sent frame %d for monitor %d to client %s (size: %d bytes)",
						frameCount, monitor.ID, client.id, len(frameData))
				}
			}
		}
		s.clientsMutex.Unlock()
		
		// Update sent counter if any clients received the frame
		if clientsReceived > 0 {
			framesSent++
			if framesSent % 30 == 0 {
				log.Printf("Monitor %d: Sent %d frames to %d clients", 
					monitor.ID, framesSent, clientsReceived)
			}
		} else if clientCount > 0 && frameCount % 10 == 0 {
			// This suggests a mapping issue
			log.Printf("Warning: No clients received frame for monitor %d despite %d clients being connected",
				monitor.ID, clientCount)
		}

		// Sleep to maintain target frame rate (30fps)
		time.Sleep(33 * time.Millisecond)
	}
}