# Hue CLI

A powerful command-line interface for controlling Philips Hue lights with support for the high-performance Entertainment API.

## Features

- 🔍 **Bridge Discovery** - Automatically find Hue bridges on your network
- 🔐 **Easy Authentication** - Simple setup with push-link authentication
- 💡 **Light Control** - Turn lights on/off, adjust brightness, change colors
- 🎨 **Entertainment API** - Stream colors to lights at up to 60 FPS via DTLS
- 🌐 **WebSocket Server** - Real-time streaming interface for external applications
- 🎭 **Scenes** - Create and activate light scenes
- 👥 **Groups** - Manage and control light groups

## Installation

### Pre-built Binaries

Download the latest release from the [Releases](../../releases) page.

### Build from Source

Requirements:
- Go 1.19 or later

```bash
git clone https://github.com/LucasionGS/hue-cli.git
cd hue-cli
go build -o bin/hue.exe
```

Or use the build script:
```bash
./build
```

## Quick Start

### 1. Discover Your Bridge

Find your Hue bridge on the network:

```bash
hue discover
```

This will show you the IP address of your bridge.

### 2. Authenticate

Press the button on your Hue bridge, then run:

```bash
hue auth
```

This creates credentials and stores them in `~/.hue-config.json`. The credentials include:
- **Username** - Used for standard API calls
- **Client Key** - Required for Entertainment API streaming (DTLS)

**Note:** For Entertainment API features, the `auth` command automatically generates a client key. If you have existing credentials without a client key, run `hue auth` again to generate one.

### 3. Start Using Hue CLI

List your lights:
```bash
hue list
```

Turn lights on:
```bash
hue on 1 2 3
```

Change color:
```bash
hue color 1 255 0 0  # Red
```

## Usage

### Basic Commands

#### Light Control

```bash
# List all lights
hue list

# Turn lights on (by ID, name, or group)
hue on 1 2 3
hue on "Living Room Light"
hue on g:Living-Room  # Turn on a group

# Turn lights off
hue off 1 2 3
hue off g:Kitchen

# Set brightness (0-254)
hue brightness 1 200
hue brightness "Desk Lamp" 150
hue brightness g:Bedroom 100

# Set RGB color (0-255 per channel)
hue color 1 255 0 0
hue color "Kitchen Light" 0 255 0
hue color g:Living-Room 0 0 255

# Set color with hex code
hue color 1 FF5733
hue color "Desk Lamp" A3C1AD

# Check authorization status
hue status
```

#### Groups

```bash
# Create a group or add lights to it
hue group add "Living Room" 1 2 3

# List all groups
hue group groups

# List lights in a specific group
hue group list "Living Room"

# Control a group (prefix with g:)
hue on g:Living-Room
hue color g:Living-Room 0 255 0
```

#### Scenes

```bash
# Create a scene by adding commands
hue scene add "Movie Time" color "Light 1" 255 100 50
hue scene add "Movie Time" brightness "Light 2" 100
hue scene add "Movie Time" off "Light 3"

# List all scenes
hue scene scenes

# List commands in a specific scene
hue scene list "Movie Time"

# Activate a scene
hue scene "Movie Time"
```

### Entertainment API

The Entertainment API enables high-speed light streaming at up to 60 FPS using DTLS protocol.

#### Setup

1. **Create an Entertainment Area** in the Hue app or via CLI:
   ```bash
   hue entertain area create "My Room" 1 2 3
   ```

2. **List Entertainment Areas**:
   ```bash
   hue entertain list
   ```

#### Built-in Effects

Stream pre-built effects to your entertainment area:

```bash
# Rainbow effect
hue entertain stream effect "My Room" rainbow

# Pulse effect (white pulsing)
hue entertain stream effect "My Room" pulse

# Wave effect (colors moving across lights)
hue entertain stream effect "My Room" wave

# Random colors
hue entertain stream effect "My Room" random
```

#### WebSocket Server

For real-time streaming from external applications, start the WebSocket server:

```bash
# Start server on default port (8080)
hue entertain stream server "My Room"

# Start server on custom port
hue entertain stream server "My Room" 9000
```

The server provides:
- **WebSocket endpoint**: `ws://localhost:8080/ws` - Send color data in real-time
- **Status endpoint**: `http://localhost:8080/status` - Get light IDs and area info
- **Documentation**: `http://localhost:8080/` - Interactive API documentation

For detailed WebSocket API documentation and examples, see [WEBSOCKET_API.md](WEBSOCKET_API.md).

#### Use Cases

- **Screen sync / Ambilight** - Sync lights with screen colors
- **Music visualization** - React to audio in real-time
- **Game integration** - Sync lights with game events
- **Custom effects** - Build your own lighting effects

## Configuration

Credentials are stored in `~/.hue-config.json`:

```json
{
  "host": "192.168.1.100",
  "username": "your-username-here",
  "id": "bridge-id",
  "clientkey": "YOUR-CLIENT-KEY-HERE"
}
```

### Configuration Fields

- **host** - Bridge IP address (without http:// prefix)
- **username** - API username for standard operations
- **id** - Bridge unique identifier
- **clientkey** - 16-byte key (32 hex characters) required for Entertainment API

## Authentication Details

### Standard Authentication

The standard authentication process creates a username that allows control of lights, groups, and scenes:

```bash
hue auth
```

1. Press the button on your Hue bridge
2. Run the command within 30 seconds
3. Credentials are saved to `~/.hue-config.json`

### Entertainment API Authentication

For Entertainment API streaming, a **client key** is required in addition to the username. The `hue auth` command automatically generates both:

```bash
hue auth
```

This generates:
- **Username** - For standard API calls
- **Client Key** - For Entertainment API DTLS encryption

The client key is used as the Pre-Shared Key (PSK) for DTLS handshake with the bridge.

### Re-authenticating

If you need to generate new credentials or add a client key to existing credentials:

```bash
hue auth
```

This will create new credentials (including client key) and update your config file.

## Command Reference

### Discovery & Setup
- `hue discover` - Find Hue bridges on network
- `hue find` - Alternative bridge discovery method
- `hue auth` - Authenticate with bridge (generates username + client key)
- `hue status` - Check authentication status

### Light Control
- `hue list` - List all lights
- `hue on <light-id/name/group>` - Turn lights on (use `g:groupname` for groups)
- `hue off <light-id/name/group>` - Turn lights off
- `hue brightness <light-id/name/group> <0-254>` - Set brightness
- `hue color <light-id/name/group> <r> <g> <b>` - Set RGB color (0-255)
- `hue color <light-id/name/group> <hex>` - Set color using hex code

### Groups
- `hue group add <name> <light-ids/names...>` - Create group or add lights
- `hue group groups` - List all groups
- `hue group list <name>` - List lights in a group
- `hue group remove <name> [index]` - Remove light from group or delete group

### Scenes
- `hue scene <name>` - Activate scene
- `hue scene add <name> <command> <light/group> <args...>` - Add command to scene
- `hue scene scenes` - List all scenes
- `hue scene list <name>` - List commands in a scene
- `hue scene remove <name> [index]` - Remove command from scene or delete scene

### Entertainment API
- `hue entertain list` - List entertainment areas
- `hue entertain area create <name> <light-ids>` - Create entertainment area
- `hue entertain area delete <id>` - Delete entertainment area
- `hue entertain stream effect <area> <effect>` - Stream built-in effect
- `hue entertain stream server <area> [port]` - Start WebSocket server

Available effects: `rainbow`, `pulse`, `wave`, `random`

## Requirements

- Philips Hue Bridge (v2) with API version 1.22 or higher
- Bridge and computer on the same network
- For Entertainment API:
  - Bridge firmware with Entertainment API support
  - Entertainment area configured (via Hue app or CLI)
  - Client key generated (automatic with `hue auth`)

## Troubleshooting

### Cannot find bridge

- Ensure your computer and bridge are on the same network
- Try `hue find` as an alternative discovery method
- Check your router's connected devices for the bridge IP
- Manually verify bridge at `http://<bridge-ip>/api/config`

### Authentication fails

- Press the bridge button first, then run `hue auth` within 30 seconds
- If already authenticated, credentials are in `~/.hue-config.json`
- To re-authenticate, delete the config file and run `hue auth` again

### Entertainment API not working

- Ensure you have a client key: Check `~/.hue-config.json` for `clientkey` field
- If missing, run `hue auth` to generate credentials with client key
- Verify entertainment area exists: `hue entertain list`
- Create an entertainment area if needed: `hue entertain area create "Room Name" 1 2 3`
- Only one Entertainment stream can be active at a time

### Lights not responding to streaming

- Verify light IDs are correct: `hue entertain list`
- Check that the entertainment area is properly configured
- Ensure no other application is using the Entertainment API
- Try stopping and restarting the streaming server

### WebSocket connection issues

- Check the server is running: Look for "✅ DTLS streaming connected"
- Verify the port is not in use by another application
- Ensure firewall allows connections on the specified port
- Check `/status` endpoint returns valid data

## Examples

### Basic Light Control

```bash
# Turn on lights 1, 2, and 3
hue on 1 2 3

# Set light 1 to red at half brightness
hue color 1 255 0 0
hue brightness 1 127

# Turn off all lights in a group
hue off g:Living-Room
```

### Create and Use Scenes

```bash
# Add commands to a scene
hue scene add "Sunset" color "Light 1" 255 100 0
hue scene add "Sunset" brightness "Light 1" 200
hue scene add "Sunset" color "Light 2" 0 100 255
hue scene add "Sunset" brightness "Light 2" 150

# Activate the scene
hue scene "Sunset"
```

### Entertainment API Streaming

```bash
# Start WebSocket server
hue entertain stream server "Gaming Room"

# In your application, connect to ws://localhost:8080/ws
# Send color data as JSON:
# {"lights": {"1": {"r": 255, "g": 0, "b": 0}}}
```

See [WEBSOCKET_API.md](WEBSOCKET_API.md) for complete WebSocket examples in multiple languages.

## Project Structure

```
hue-cli/
├── main.go                  # CLI entry point and basic commands
├── entertainment.go         # Entertainment API commands
├── entertainment_dtls.go    # DTLS streaming implementation
├── entertainment_websocket.go # WebSocket server
├── test-websocket.html      # WebSocket test interface
├── stream_example.py        # Python streaming example
├── WEBSOCKET_API.md         # WebSocket API documentation
└── bin/                     # Compiled binaries
```

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

## License

[MIT License](LICENSE)

## Acknowledgments

- Built with [huego](https://github.com/amimof/huego) for Hue API interaction
- Uses [cobra](https://github.com/spf13/cobra) for CLI framework
- Uses [pion/dtls](https://github.com/pion/dtls) for DTLS streaming
- Uses [gorilla/websocket](https://github.com/gorilla/websocket) for WebSocket server

## Resources

- [Philips Hue API Documentation](https://developers.meethue.com/)
- [Hue Entertainment API Specification](https://developers.meethue.com/develop/hue-entertainment/)
- [WebSocket API Documentation](WEBSOCKET_API.md)
