package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/pion/dtls/v2"
)

// RGB represents an RGB color
type RGB struct {
	R, G, B uint8
}

// DTLSStream represents an active DTLS streaming connection to the bridge
type DTLSStream struct {
	conn        *dtls.Conn
	area        *EntertainmentArea
	sequenceNum uint8
	isActive    bool
	stopChan    chan struct{}
	updateRate  time.Duration // Time between updates
}

// createDTLSStreamConnection establishes a DTLS connection to the bridge for entertainment streaming
func createDTLSStreamConnection(area *EntertainmentArea) (*DTLSStream, error) {
	// Temporary: Force HTTP fallback for testing
	// return nil, fmt.Errorf("DTLS temporarily disabled for testing")

	bridgeConfig, err := loadBridgeConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load bridge config: %v", err)
	}

	if bridgeConfig.ClientKey == "" {
		return nil, fmt.Errorf("no clientkey found - run 'hue auth' to generate streaming credentials")
	}

	// Convert hex clientkey to bytes
	clientKeyBytes, err := hex.DecodeString(bridgeConfig.ClientKey)
	if err != nil {
		return nil, fmt.Errorf("invalid clientkey format: %v", err)
	}

	// Parse bridge host to get IP
	host := bridgeConfig.Host
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimPrefix(host, "https://")

	// Entertainment streaming port is 2100
	addr := fmt.Sprintf("%s:2100", host)

	// Resolve the address
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve address: %v", err)
	}

	// Create UDP connection
	udpConn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create UDP connection: %v", err)
	}

	// Configure DTLS with PSK (Pre-Shared Key)
	// Identity is the username, PSK is the clientkey bytes
	config := &dtls.Config{
		PSK: func(hint []byte) ([]byte, error) {
			return clientKeyBytes, nil
		},
		PSKIdentityHint:      []byte(bridgeConfig.Username),
		CipherSuites:         []dtls.CipherSuiteID{dtls.TLS_PSK_WITH_AES_128_GCM_SHA256},
		ExtendedMasterSecret: dtls.RequireExtendedMasterSecret,
	}

	// Establish DTLS connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dtlsConn, err := dtls.ClientWithContext(ctx, udpConn, config)
	if err != nil {
		udpConn.Close()
		return nil, fmt.Errorf("DTLS handshake failed: %v", err)
	}

	stream := &DTLSStream{
		conn:        dtlsConn,
		area:        area,
		sequenceNum: 0,
		isActive:    true,
		stopChan:    make(chan struct{}),
		updateRate:  16 * time.Millisecond, // ~60 fps
	}

	return stream, nil
}

// SendColors sends RGB color data to lights via DTLS streaming
func (s *DTLSStream) SendColors(lightColors map[string]RGB) error {
	if !s.isActive {
		return fmt.Errorf("stream not active")
	}

	// Build entertainment protocol message
	buf := s.buildStreamMessage(lightColors)

	// Send via DTLS
	_, err := s.conn.Write(buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to send DTLS message: %v", err)
	}

	// Increment sequence number for next message
	s.sequenceNum++

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// buildStreamMessage creates the binary entertainment protocol message
func (s *DTLSStream) buildStreamMessage(lightColors map[string]RGB) *bytes.Buffer {
	buf := new(bytes.Buffer)

	// Header: "HueStream" (9 bytes)
	buf.WriteString("HueStream")

	// API Version: 0x01, 0x00 (2 bytes) - Version 1.0
	buf.WriteByte(0x01)
	buf.WriteByte(0x00)

	// Sequence number (1 byte) - increments with each message
	buf.WriteByte(s.sequenceNum)

	// Reserved: 0x00 0x00 (2 bytes)
	buf.WriteByte(0x00)
	buf.WriteByte(0x00)

	// Color space: 0x00 = RGB (1 byte)
	buf.WriteByte(0x00)

	// Reserved: 0x00 (1 byte)
	buf.WriteByte(0x00)

	// Write light data (each light is 9 bytes: 1 byte type + 2 bytes ID + 6 bytes RGB)
	for _, lightID := range s.area.Lights {
		rgb, ok := lightColors[lightID]
		if !ok {
			// If no color specified, use black (off)
			rgb = RGB{R: 0, G: 0, B: 0}
		}

		// Type: 0x00 = Light (1 byte)
		buf.WriteByte(0x00)

		// Light ID as uint16 (2 bytes, big-endian)
		// Parse the string light ID to integer
		var lightIDNum uint16
		fmt.Sscanf(lightID, "%d", &lightIDNum)
		binary.Write(buf, binary.BigEndian, lightIDNum)

		// RGB values as uint16 (big-endian)
		// Scale 0-255 to 0-65535 by shifting left 8 bits
		binary.Write(buf, binary.BigEndian, uint16(rgb.R)<<8)
		binary.Write(buf, binary.BigEndian, uint16(rgb.G)<<8)
		binary.Write(buf, binary.BigEndian, uint16(rgb.B)<<8)
	}

	return buf
}

// StreamEffect runs a visual effect using DTLS streaming
func (s *DTLSStream) StreamEffect(effectName string, durationSec int) error {
	fmt.Printf("Streaming via DTLS at ~60fps for %d seconds...\n", durationSec)

	switch effectName {
	case "rainbow":
		return s.streamRainbowEffect(durationSec)
	case "pulse":
		return s.streamPulseEffect(durationSec)
	case "wave":
		return s.streamWaveEffect(durationSec)
	case "random":
		return s.streamRandomEffect(durationSec)
	default:
		return fmt.Errorf("unknown effect: %s", effectName)
	}
}

// streamRainbowEffect cycles through rainbow colors
func (s *DTLSStream) streamRainbowEffect(durationSec int) error {
	startTime := time.Now()
	ticker := time.NewTicker(s.updateRate)
	defer ticker.Stop()

	hue := 0.0
	for time.Since(startTime) < time.Duration(durationSec)*time.Second {
		<-ticker.C

		// Create color map
		colors := make(map[string]RGB)
		for _, lightID := range s.area.Lights {
			r, g, b := hsvToRGB(hue, 1.0, 1.0)
			colors[lightID] = RGB{R: r, G: g, B: b}
		}

		if err := s.SendColors(colors); err != nil {
			return err
		}

		// Increment hue for smooth rainbow
		hue += 2.0
		if hue >= 360.0 {
			hue = 0.0
		}
	}

	return nil
}

// streamPulseEffect creates a pulsing white light
func (s *DTLSStream) streamPulseEffect(durationSec int) error {
	startTime := time.Now()
	ticker := time.NewTicker(s.updateRate)
	defer ticker.Stop()

	phase := 0.0
	for time.Since(startTime) < time.Duration(durationSec)*time.Second {
		<-ticker.C

		// Sine wave for smooth pulsing (0.5 to 1.0 range)
		brightness := 0.5 + 0.5*((1.0+float64(phase))/2.0)
		intensity := uint8(brightness * 255)

		colors := make(map[string]RGB)
		for _, lightID := range s.area.Lights {
			colors[lightID] = RGB{R: intensity, G: intensity, B: intensity}
		}

		if err := s.SendColors(colors); err != nil {
			return err
		}

		// Update phase for sine wave
		phase += 0.1
		if phase > 360.0 {
			phase = 0.0
		}
	}

	return nil
}

// streamWaveEffect creates a color wave across lights
func (s *DTLSStream) streamWaveEffect(durationSec int) error {
	startTime := time.Now()
	ticker := time.NewTicker(s.updateRate)
	defer ticker.Stop()

	offset := 0.0
	for time.Since(startTime) < time.Duration(durationSec)*time.Second {
		<-ticker.C

		colors := make(map[string]RGB)
		numLights := len(s.area.Lights)

		for i, lightID := range s.area.Lights {
			// Each light gets a different hue based on position + offset
			hue := (offset + float64(i)*360.0/float64(numLights))
			if hue >= 360.0 {
				hue -= 360.0
			}

			r, g, b := hsvToRGB(hue, 1.0, 1.0)
			colors[lightID] = RGB{R: r, G: g, B: b}
		}

		if err := s.SendColors(colors); err != nil {
			return err
		}

		// Move wave forward
		offset += 3.0
		if offset >= 360.0 {
			offset = 0.0
		}
	}

	return nil
}

// streamRandomEffect creates random color flashing
func (s *DTLSStream) streamRandomEffect(durationSec int) error {
	startTime := time.Now()
	ticker := time.NewTicker(200 * time.Millisecond) // Slower for random
	defer ticker.Stop()

	for time.Since(startTime) < time.Duration(durationSec)*time.Second {
		<-ticker.C

		colors := make(map[string]RGB)
		for _, lightID := range s.area.Lights {
			// Random hue for each light
			hue := float64(time.Now().UnixNano() % 360)
			r, g, b := hsvToRGB(hue, 1.0, 1.0)
			colors[lightID] = RGB{R: r, G: g, B: b}
		}

		if err := s.SendColors(colors); err != nil {
			return err
		}
	}

	return nil
}

// Close closes the DTLS connection
func (s *DTLSStream) Close() error {
	s.isActive = false
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

// hsvToRGB converts HSV color space to RGB
// h: 0-360, s: 0-1, v: 0-1
// returns r, g, b: 0-255
func hsvToRGB(h, s, v float64) (uint8, uint8, uint8) {
	// Normalize hue to 0-360
	for h < 0 {
		h += 360
	}
	for h >= 360 {
		h -= 360
	}

	c := v * s
	x := c * (1 - abs(mod(h/60.0, 2)-1))
	m := v - c

	var r, g, b float64

	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}

	return uint8((r + m) * 255), uint8((g + m) * 255), uint8((b + m) * 255)
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func mod(x, y float64) float64 {
	return x - y*float64(int(x/y))
}
