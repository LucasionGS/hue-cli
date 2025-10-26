package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for local development
	},
}

// WebSocketServer manages WebSocket connections for real-time light streaming
type WebSocketServer struct {
	port            int
	dtlsStream      *DTLSStream
	area            *EntertainmentArea
	server          *http.Server
	isStreaming     bool
	mutex           sync.Mutex
	stopChan        chan struct{}
	lastMessageTime time.Time
	lastColors      map[string]RGB
}

// LightColorMessage represents a WebSocket message with light colors
type LightColorMessage struct {
	// Map of light ID to RGB color
	Lights map[string]struct {
		R uint8 `json:"r"`
		G uint8 `json:"g"`
		B uint8 `json:"b"`
	} `json:"lights"`
}

// StartWebSocketServer starts a WebSocket server for real-time streaming
func StartWebSocketServer(area *EntertainmentArea, port int) error {
	server := &WebSocketServer{
		port:            port,
		area:            area,
		stopChan:        make(chan struct{}),
		lastMessageTime: time.Now(),
		lastColors:      make(map[string]RGB),
	}

	// Activate streaming on the bridge
	if err := activateStreaming(area.ID); err != nil {
		return fmt.Errorf("failed to activate streaming: %v", err)
	}

	// Create DTLS connection
	dtlsStream, err := createDTLSStreamConnection(area)
	if err != nil {
		deactivateStreaming(area.ID)
		time.Sleep(1000)
		dtlsStream, err = createDTLSStreamConnection(area)
		if err != nil {
			deactivateStreaming(area.ID)
			return fmt.Errorf("failed to create DTLS connection: %v", err)
		}
	}
	server.dtlsStream = dtlsStream

	// Start HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", server.handleWebSocket)
	mux.HandleFunc("/status", server.handleStatus)
	mux.HandleFunc("/", server.handleIndex)

	server.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	fmt.Printf("✅ DTLS streaming connected\n")
	fmt.Printf("🌐 WebSocket server starting on http://localhost:%d\n", port)
	fmt.Printf("📡 WebSocket endpoint: ws://localhost:%d/ws\n", port)
	fmt.Printf("📊 Status endpoint: http://localhost:%d/status\n", port)
	fmt.Printf("🔄 Keep-alive: Sending updates every 5 seconds\n")
	fmt.Printf("\nPress Ctrl+C to stop streaming...\n\n")

	// Start keepalive goroutine
	go server.keepAlive()

	// Start server
	if err := server.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		server.cleanup()
		return err
	}

	return nil
}

// handleWebSocket handles WebSocket connections for streaming
func (s *WebSocketServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("WebSocket upgrade error: %v\n", err)
		return
	}
	defer conn.Close()

	fmt.Printf("✅ WebSocket client connected from %s\n", r.RemoteAddr)

	s.mutex.Lock()
	s.isStreaming = true
	s.mutex.Unlock()

	for {
		var msg LightColorMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				fmt.Printf("WebSocket error: %v\n", err)
			}
			break
		}

		// Convert message to RGB map
		colors := make(map[string]RGB)
		for lightID, color := range msg.Lights {
			colors[lightID] = RGB{
				R: color.R,
				G: color.G,
				B: color.B,
			}
		}

		// Send to bridge via DTLS
		if err := s.dtlsStream.SendColors(colors); err != nil {
			fmt.Printf("Error sending colors: %v\n", err)
			conn.WriteJSON(map[string]string{"error": err.Error()})
			break
		}

		// Update last message time and colors
		s.mutex.Lock()
		s.lastMessageTime = time.Now()
		s.lastColors = colors
		s.mutex.Unlock()
	}

	fmt.Printf("❌ WebSocket client disconnected\n")
	s.mutex.Lock()
	s.isStreaming = false
	s.mutex.Unlock()
}

// handleStatus returns the current streaming status
func (s *WebSocketServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers to allow all origins
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle preflight request
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	s.mutex.Lock()
	isStreaming := s.isStreaming
	s.mutex.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"streaming": isStreaming,
		"area":      s.area.Name,
		"lights":    s.area.Lights,
		"port":      s.port,
	})
}

// handleIndex serves a simple HTML page with usage instructions
func (s *WebSocketServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Hue Entertainment Streaming</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 800px; margin: 50px auto; padding: 20px; }
        code { background: #f4f4f4; padding: 2px 6px; border-radius: 3px; }
        pre { background: #f4f4f4; padding: 15px; border-radius: 5px; overflow-x: auto; }
        .status { padding: 10px; margin: 10px 0; border-radius: 5px; }
        .active { background: #d4edda; border: 1px solid #c3e6cb; }
        .inactive { background: #f8d7da; border: 1px solid #f5c6cb; }
    </style>
</head>
<body>
    <h1>🎨 Hue Entertainment Streaming Server</h1>
    <div class="status active">
        <strong>Status:</strong> Server is running<br>
        <strong>Area:</strong> %s<br>
        <strong>Lights:</strong> %v
    </div>
    
    <h2>WebSocket API</h2>
    <p>Connect to: <code>ws://localhost:%d/ws</code></p>
    
    <h3>Message Format</h3>
    <pre>{
  "lights": {
    "17": { "r": 255, "g": 0, "b": 0 },
    "18": { "r": 0, "g": 255, "b": 0 },
    "16": { "r": 0, "g": 0, "b": 255 }
  }
}</pre>

    <h3>Example JavaScript</h3>
    <pre>const ws = new WebSocket('ws://localhost:%d/ws');

ws.onopen = () => {
  console.log('Connected to Hue streaming');
  
  // Send colors to lights
  ws.send(JSON.stringify({
    lights: {
      '17': { r: 255, g: 0, b: 0 },
      '18': { r: 0, g: 255, b: 0 }
    }
  }));
};

ws.onerror = (error) => {
  console.error('WebSocket error:', error);
};</pre>

    <h3>Example Python</h3>
    <pre>import websocket
import json

ws = websocket.WebSocket()
ws.connect('ws://localhost:%d/ws')

# Send colors
ws.send(json.dumps({
    'lights': {
        '17': {'r': 255, 'g': 0, 'b': 0},
        '18': {'r': 0, 'g': 255, 'b': 0}
    }
}))

ws.close()</pre>

    <h3>Available Lights</h3>
    <ul>
`, s.area.Name, s.area.Lights, s.port, s.port, s.port)

	for _, lightID := range s.area.Lights {
		html += fmt.Sprintf("        <li>Light ID: <code>%s</code></li>\n", lightID)
	}

	html += `    </ul>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// keepAlive sends periodic messages to keep the DTLS connection alive
// The bridge closes the connection after 10 seconds of inactivity
func (s *WebSocketServer) keepAlive() {
	// Send keep-alive every 5 seconds (safely within the 10 second timeout)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if s.dtlsStream == nil {
				continue
			}

			s.mutex.Lock()
			lastColors := s.lastColors
			s.mutex.Unlock()

			// Send last known colors to maintain current state
			var colors map[string]RGB
			if len(lastColors) > 0 {
				// Resend last colors to keep lights in current state
				colors = lastColors
			} else {
				// Initialize with black if no colors have been sent yet
				colors = make(map[string]RGB)
				for _, lightID := range s.area.Lights {
					colors[lightID] = RGB{R: 0, G: 0, B: 0}
				}
			}

			if err := s.dtlsStream.SendColors(colors); err != nil {
				fmt.Printf("⚠️ Keep-alive failed: %v\n", err)
			} else {
				// fmt.Printf("✓ Keep-alive sent (%d lights)\n", len(colors))
				s.mutex.Lock()
				s.lastMessageTime = time.Now()
				s.mutex.Unlock()
			}
		case <-s.stopChan:
			return
		}
	}
}

// cleanup closes connections and deactivates streaming
func (s *WebSocketServer) cleanup() {
	close(s.stopChan)
	if s.dtlsStream != nil {
		s.dtlsStream.Close()
	}
	deactivateStreaming(s.area.ID)
}
