# Hue Entertainment WebSocket API

The Hue CLI provides a high-performance WebSocket server that allows external applications to stream color data to Philips Hue lights in real-time at up to 60 frames per second.

## Overview

The WebSocket server establishes a DTLS connection to your Hue bridge and maintains it with automatic keep-alive messages. External applications can connect via WebSocket and send JSON messages with light colors, which are immediately streamed to the lights.

## Getting Started

### 1. Start the WebSocket Server

First, you need to start the server for a specific entertainment area:

```bash
hue entertain stream server "Area Name"
```

Or specify a custom port (default is 8080):

```bash
hue entertain stream server "Area Name" 9000
```

The server will output:
```
✅ DTLS streaming connected
🌐 WebSocket server starting on http://localhost:8080
📡 WebSocket endpoint: ws://localhost:8080/ws
📊 Status endpoint: http://localhost:8080/status
🔄 Keep-alive: Sending updates every 5 seconds

Press Ctrl+C to stop streaming...
```

### 2. Get Available Light IDs

Before sending colors, you need to know which light IDs are in your entertainment area. Query the status endpoint:

```bash
curl http://localhost:8080/status
```

Response:
```json
{
  "streaming": false,
  "area": "Room",
  "lights": ["17", "18", "16", "8", "10", "9"],
  "port": 8080
}
```

### 3. Connect and Stream

Connect to `ws://localhost:8080/ws` and start sending color data.

## WebSocket Protocol

### Connection

```javascript
const ws = new WebSocket('ws://localhost:8080/ws');

ws.onopen = () => {
  console.log('Connected to Hue Entertainment');
};

ws.onerror = (error) => {
  console.error('WebSocket error:', error);
};

ws.onclose = () => {
  console.log('Disconnected');
};
```

### Message Format

Send JSON messages with light colors in RGB format (0-255 per channel):

```json
{
  "lights": {
    "17": { "r": 255, "g": 0, "b": 0 },
    "18": { "r": 0, "g": 255, "b": 0 },
    "16": { "r": 0, "g": 0, "b": 255 }
  }
}
```

#### Fields

- **`lights`** (object, required): Map of light IDs to RGB colors
  - **Key**: Light ID as a string (e.g., `"17"`)
  - **Value**: Object with RGB components
    - **`r`** (number, 0-255): Red component
    - **`g`** (number, 0-255): Green component
    - **`b`** (number, 0-255): Blue component

#### Notes

- You don't need to include all lights in every message - only include the lights you want to update
- RGB values are integers from 0-255
- Messages are processed immediately and sent to the bridge via DTLS
- For best results, send updates at 25-60 FPS

### Error Responses

If an error occurs, the server will send a JSON message:

```json
{
  "error": "Error description"
}
```

## Examples

### JavaScript (Browser)

```javascript
const ws = new WebSocket('ws://localhost:8080/ws');

ws.onopen = () => {
  // Rainbow effect
  let hue = 0;
  setInterval(() => {
    const rgb = hsvToRgb(hue, 1, 1);
    
    ws.send(JSON.stringify({
      lights: {
        '17': rgb,
        '18': rgb,
        '16': rgb
      }
    }));
    
    hue = (hue + 2) % 360;
  }, 1000 / 60); // 60 FPS
};

function hsvToRgb(h, s, v) {
  const c = v * s;
  const x = c * (1 - Math.abs((h / 60) % 2 - 1));
  const m = v - c;
  
  let r, g, b;
  if (h < 60) { r = c; g = x; b = 0; }
  else if (h < 120) { r = x; g = c; b = 0; }
  else if (h < 180) { r = 0; g = c; b = x; }
  else if (h < 240) { r = 0; g = x; b = c; }
  else if (h < 300) { r = x; g = 0; b = c; }
  else { r = c; g = 0; b = x; }
  
  return {
    r: Math.floor((r + m) * 255),
    g: Math.floor((g + m) * 255),
    b: Math.floor((b + m) * 255)
  };
}
```

### Python

```python
import websocket
import json
import time
import math

# Connect to server
ws = websocket.WebSocket()
ws.connect('ws://localhost:8080/ws')

# Get light IDs from status endpoint
import urllib.request
with urllib.request.urlopen('http://localhost:8080/status') as response:
    status = json.loads(response.read())
    lights = status['lights']

# Pulse effect
phase = 0
while True:
    brightness = int((math.sin(phase) * 0.5 + 0.5) * 255)
    
    message = {
        'lights': {
            light_id: {'r': brightness, 'g': brightness, 'b': brightness}
            for light_id in lights
        }
    }
    
    ws.send(json.dumps(message))
    phase += 0.1
    time.sleep(1/60)  # 60 FPS
```

### Node.js

```javascript
const WebSocket = require('ws');

const ws = new WebSocket('ws://localhost:8080/ws');

ws.on('open', () => {
  console.log('Connected');
  
  // Wave effect
  let offset = 0;
  const lights = ['17', '18', '16', '8', '10', '9'];
  
  setInterval(() => {
    const colors = {};
    
    lights.forEach((lightId, index) => {
      const hue = (offset + index * 60) % 360;
      colors[lightId] = hsvToRgb(hue, 1, 1);
    });
    
    ws.send(JSON.stringify({ lights: colors }));
    offset = (offset + 2) % 360;
  }, 1000 / 60);
});

ws.on('error', (error) => {
  console.error('Error:', error);
});
```

### C# / Unity

```csharp
using System;
using System.Net.WebSockets;
using System.Text;
using System.Text.Json;
using System.Threading;
using System.Threading.Tasks;

public class HueStreamer
{
    private ClientWebSocket ws;
    private string[] lightIds;

    public async Task Connect()
    {
        ws = new ClientWebSocket();
        await ws.ConnectAsync(new Uri("ws://localhost:8080/ws"), CancellationToken.None);
        
        // Get light IDs from status endpoint
        using var client = new HttpClient();
        var status = await client.GetStringAsync("http://localhost:8080/status");
        var statusObj = JsonSerializer.Deserialize<StatusResponse>(status);
        lightIds = statusObj.lights;
    }

    public async Task SendColors(Dictionary<string, RGB> colors)
    {
        var message = new { lights = colors };
        var json = JsonSerializer.Serialize(message);
        var bytes = Encoding.UTF8.GetBytes(json);
        
        await ws.SendAsync(
            new ArraySegment<byte>(bytes), 
            WebSocketMessageType.Text, 
            true, 
            CancellationToken.None
        );
    }

    public void Close()
    {
        ws?.Dispose();
    }
}

public class RGB
{
    public int r { get; set; }
    public int g { get; set; }
    public int b { get; set; }
}

public class StatusResponse
{
    public string[] lights { get; set; }
    public string area { get; set; }
    public int port { get; set; }
}
```

## HTTP Endpoints

### GET /status

Returns the current server status and entertainment area information.

**Response:**
```json
{
  "streaming": false,
  "area": "Room",
  "lights": ["17", "18", "16", "8", "10", "9"],
  "port": 8080
}
```

### GET /

Returns an HTML page with API documentation and usage examples.

## Best Practices

### Performance

1. **Frame Rate**: Send updates at 25-60 FPS for smooth effects
   - Lower frame rates (10-25 FPS) work for slower effects
   - Higher frame rates (50-60 FPS) are best for fast-moving effects

2. **Partial Updates**: Only include lights you want to change
   ```json
   // Only update two lights
   {
     "lights": {
       "17": { "r": 255, "g": 0, "b": 0 },
       "18": { "r": 0, "g": 255, "b": 0 }
     }
   }
   ```

3. **Keep Connection Alive**: The server automatically sends keep-alive messages, so your connection won't time out during idle periods

### Error Handling

Always handle WebSocket errors and reconnection:

```javascript
ws.onerror = (error) => {
  console.error('WebSocket error:', error);
};

ws.onclose = () => {
  console.log('Connection closed, reconnecting...');
  setTimeout(connect, 1000);
};
```

### Color Conversion

If working with HSV or other color spaces, convert to RGB before sending:

```javascript
function hsvToRgb(h, s, v) {
  // h: 0-360, s: 0-1, v: 0-1
  const c = v * s;
  const x = c * (1 - Math.abs((h / 60) % 2 - 1));
  const m = v - c;
  
  let r, g, b;
  if (h < 60) { r = c; g = x; b = 0; }
  else if (h < 120) { r = x; g = c; b = 0; }
  else if (h < 180) { r = 0; g = c; b = x; }
  else if (h < 240) { r = 0; g = x; b = c; }
  else if (h < 300) { r = x; g = 0; b = c; }
  else { r = c; g = 0; b = x; }
  
  return {
    r: Math.floor((r + m) * 255),
    g: Math.floor((g + m) * 255),
    b: Math.floor((b + m) * 255)
  };
}
```

## Common Use Cases

### Screen Sync / Ambilight

Capture screen colors and send to lights based on screen regions:

```javascript
// Pseudo-code
function syncScreen() {
  const screenCapture = captureScreen();
  const regions = divideIntoRegions(screenCapture, lights.length);
  
  const colors = {};
  regions.forEach((region, index) => {
    const avgColor = getAverageColor(region);
    colors[lights[index]] = avgColor;
  });
  
  ws.send(JSON.stringify({ lights: colors }));
}

setInterval(syncScreen, 1000 / 30); // 30 FPS
```

### Music Visualization

React to audio input:

```javascript
// Get audio data from analyzer
const dataArray = new Uint8Array(analyser.frequencyBinCount);
analyser.getByteFrequencyData(dataArray);

// Map frequency bands to lights
const colors = {};
lights.forEach((lightId, index) => {
  const freqIndex = Math.floor(index * dataArray.length / lights.length);
  const intensity = dataArray[freqIndex] / 255;
  
  colors[lightId] = {
    r: Math.floor(intensity * 255),
    g: Math.floor((1 - intensity) * 128),
    b: Math.floor(intensity * 200)
  };
});

ws.send(JSON.stringify({ lights: colors }));
```

### Game Integration

Sync lights with game events:

```javascript
// React to game events
gameEvents.on('healthLow', () => {
  // Flash red
  const red = { r: 255, g: 0, b: 0 };
  const colors = Object.fromEntries(lights.map(id => [id, red]));
  ws.send(JSON.stringify({ lights: colors }));
});

gameEvents.on('levelComplete', () => {
  // Victory animation
  animateVictory();
});
```

## Troubleshooting

### Connection Refused

- Ensure the server is running: `hue entertain stream server "Area Name"`
- Check the port is correct (default is 8080)
- Verify nothing else is using the port

### Lights Not Responding

- Verify you're using the correct light IDs from `/status`
- Check RGB values are integers 0-255
- Ensure the entertainment area is properly configured in the Hue app

### Connection Drops

- The server automatically maintains the connection with keep-alive messages
- If the server crashes, restart it and reconnect your client
- Check your network stability

### CORS Errors (Browser)

- The server allows all origins by default
- If still seeing CORS errors, try loading your page from a local server instead of `file://`

## Security Notes

- The WebSocket server runs locally and accepts connections from any origin
- Only run the server on trusted networks
- The server requires the Hue bridge to be on the same network
- Authentication is handled by the CLI's bridge credentials

## Additional Resources

- See `test-websocket.html` for a complete browser-based example
- Visit `http://localhost:8080/` when the server is running for interactive documentation
- Check the main README for Hue Entertainment API setup instructions
