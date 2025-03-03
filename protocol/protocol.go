// Package protocol defines the core UltraRDP protocol functionality
package protocol

import (
	"encoding/binary"
	"io"
	"time"
)

// Constants for the protocol
const (
	// Protocol version
	ProtocolVersion = 1

	// Packet types
	PacketTypeHandshake      = 0x01
	PacketTypeVideoFrame     = 0x02
	PacketTypeAudioFrame     = 0x03
	PacketTypeMouseMove      = 0x04
	PacketTypeMouseButton    = 0x05
	PacketTypeKeyboard       = 0x06
	PacketTypeMonitorConfig  = 0x07
	PacketTypePing           = 0x08
	PacketTypePong           = 0x09
	PacketTypeQualityControl = 0x0A
)

// Packet represents a basic protocol packet
type Packet struct {
	Type      byte
	Timestamp int64 // Unix timestamp in nanoseconds
	Length    uint32
	Payload   []byte
}

// EncodePacket writes a packet to the given writer
func EncodePacket(w io.Writer, packet *Packet) error {
	// Write packet type
	if err := binary.Write(w, binary.LittleEndian, packet.Type); err != nil {
		return err
	}

	// Write timestamp
	if err := binary.Write(w, binary.LittleEndian, packet.Timestamp); err != nil {
		return err
	}

	// Write payload length
	if err := binary.Write(w, binary.LittleEndian, packet.Length); err != nil {
		return err
	}

	// Write payload
	if packet.Length > 0 {
		if _, err := w.Write(packet.Payload); err != nil {
			return err
		}
	}

	return nil
}

// DecodePacket reads a packet from the given reader
func DecodePacket(r io.Reader) (*Packet, error) {
	packet := &Packet{}

	// Read packet type
	if err := binary.Read(r, binary.LittleEndian, &packet.Type); err != nil {
		return nil, err
	}

	// Read timestamp
	if err := binary.Read(r, binary.LittleEndian, &packet.Timestamp); err != nil {
		return nil, err
	}

	// Read payload length
	if err := binary.Read(r, binary.LittleEndian, &packet.Length); err != nil {
		return nil, err
	}

	// Read payload
	if packet.Length > 0 {
		packet.Payload = make([]byte, packet.Length)
		if _, err := io.ReadFull(r, packet.Payload); err != nil {
			return nil, err
		}
	}

	return packet, nil
}

// NewPacket creates a new packet with the current timestamp
func NewPacket(packetType byte, payload []byte) *Packet {
	return &Packet{
		Type:      packetType,
		Timestamp: time.Now().UnixNano(),
		Length:    uint32(len(payload)),
		Payload:   payload,
	}
}

// MonitorInfo represents information about a single monitor
type MonitorInfo struct {
	ID        uint32
	Width     uint32
	Height    uint32
	PositionX uint32
	PositionY uint32
	Primary   bool
}

// MonitorConfig represents the configuration of all monitors
type MonitorConfig struct {
	MonitorCount uint32
	Monitors     []MonitorInfo
}

// EncodeMonitorConfig encodes a monitor configuration to bytes
func EncodeMonitorConfig(config *MonitorConfig) []byte {
	// Calculate size: 4 bytes for count + size of each monitor info
	size := 4 + config.MonitorCount*24 // 24 bytes per monitor (4+4+4+4+4+4)
	buf := make([]byte, size)

	// Write monitor count
	binary.LittleEndian.PutUint32(buf[0:4], config.MonitorCount)

	// Write each monitor info
	offset := 4
	for _, monitor := range config.Monitors {
		binary.LittleEndian.PutUint32(buf[offset:offset+4], monitor.ID)
		offset += 4
		binary.LittleEndian.PutUint32(buf[offset:offset+4], monitor.Width)
		offset += 4
		binary.LittleEndian.PutUint32(buf[offset:offset+4], monitor.Height)
		offset += 4
		binary.LittleEndian.PutUint32(buf[offset:offset+4], uint32(monitor.PositionX))
		offset += 4
		binary.LittleEndian.PutUint32(buf[offset:offset+4], uint32(monitor.PositionY))
		offset += 4

		// Encode boolean as a byte
		if monitor.Primary {
			buf[offset] = 1
		} else {
			buf[offset] = 0
		}
		offset += 4 // Using 4 bytes for alignment
	}

	return buf
}

// DecodeMonitorConfig decodes a monitor configuration from bytes
func DecodeMonitorConfig(data []byte) (*MonitorConfig, error) {
	if len(data) < 4 {
		return nil, io.ErrUnexpectedEOF
	}

	config := &MonitorConfig{}

	// Read monitor count
	config.MonitorCount = binary.LittleEndian.Uint32(data[0:4])

	// Check if data length is sufficient
	expectedSize := 4 + config.MonitorCount*24
	if uint32(len(data)) < expectedSize {
		return nil, io.ErrUnexpectedEOF
	}

	// Read each monitor info
	config.Monitors = make([]MonitorInfo, config.MonitorCount)
	offset := 4

	for i := uint32(0); i < config.MonitorCount; i++ {
		monitor := &config.Monitors[i]

		monitor.ID = binary.LittleEndian.Uint32(data[offset : offset+4])
		offset += 4
		monitor.Width = binary.LittleEndian.Uint32(data[offset : offset+4])
		offset += 4
		monitor.Height = binary.LittleEndian.Uint32(data[offset : offset+4])
		offset += 4
		monitor.PositionX = binary.LittleEndian.Uint32(data[offset : offset+4])
		offset += 4
		monitor.PositionY = binary.LittleEndian.Uint32(data[offset : offset+4])
		offset += 4

		// Decode boolean from byte
		monitor.Primary = data[offset] == 1
		offset += 4 // Using 4 bytes for alignment
	}

	return config, nil
}