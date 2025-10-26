package main

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/amimof/huego"
	"github.com/spf13/cobra"
)

// Entertainment area represents a configured entertainment area
type EntertainmentArea struct {
	ID        string              `json:"id,omitempty"`
	Name      string              `json:"name"`
	Type      string              `json:"type"` // "entertainment"
	Lights    []string            `json:"lights"`
	Locations map[string]Location `json:"locations,omitempty"` // light ID -> location
	Stream    *StreamConfig       `json:"stream,omitempty"`
}

type Location struct {
	X float32 `json:"x"` // -1.0 to 1.0
	Y float32 `json:"y"` // -1.0 to 1.0
	Z float32 `json:"z"` // -1.0 to 1.0
}

type StreamConfig struct {
	ProxyMode string `json:"proxymode"` // "auto" or "manual"
	ProxyNode string `json:"proxynode,omitempty"`
	Active    bool   `json:"active"`
	Owner     string `json:"owner,omitempty"`
}

// EntertainmentConfig stores entertainment area configurations
type EntertainmentConfig struct {
	Areas []EntertainmentArea `json:"areas"`
}

var entertainmentFile string

func init() {
	homeDir, _ := os.UserHomeDir()
	entertainmentFile = filepath.Join(homeDir, ".hue-entertainment.json")
}

// Entertainment commands
var entertainCmd = &cobra.Command{
	Use:   "entertain",
	Short: "Entertainment API for high-speed streaming",
	Long:  `Use the Hue Entertainment API for low-latency, high-frequency light updates (up to 60 updates/sec). Ideal for music sync, gaming, and dynamic effects.`,
}

func init() {
	entertainCmd.AddCommand(entertainAreaCmd)
	entertainCmd.AddCommand(entertainStreamCmd)
	entertainCmd.AddCommand(entertainListCmd)
}

var entertainAreaCmd = &cobra.Command{
	Use:   "area",
	Short: "Manage entertainment areas",
	Long:  `Create and manage entertainment areas (zones) for streaming.`,
}

func init() {
	entertainAreaCmd.AddCommand(entertainAreaCreateCmd)
	entertainAreaCmd.AddCommand(entertainAreaDeleteCmd)
	entertainAreaCmd.AddCommand(entertainAreaListCmd)
}

var entertainAreaCreateCmd = &cobra.Command{
	Use:   "create [name] [light-ids...]",
	Short: "Create an entertainment area",
	Long: `Create a new entertainment area with specified lights.
	
Examples:
  hue entertain area create "Gaming Setup" 1 2 3
  hue entertain area create "TV Backlight" 4 5 6 7`,
	Args: cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		areaName := args[0]
		lightIDs := args[1:]

		// Validate lights exist
		allLights, err := bridge.GetLights()
		if err != nil {
			fmt.Printf("Error getting lights: %v\n", err)
			return
		}

		validLightIDs := []string{}
		for _, idStr := range lightIDs {
			found := false
			for _, light := range allLights {
				if fmt.Sprintf("%d", light.ID) == idStr {
					validLightIDs = append(validLightIDs, idStr)
					found = true
					break
				}
			}
			if !found {
				fmt.Printf("Warning: Light ID %s not found\n", idStr)
			}
		}

		if len(validLightIDs) == 0 {
			fmt.Println("No valid lights found")
			return
		}

		// Create entertainment configuration on the bridge
		groupID, err := createEntertainmentGroup(areaName, validLightIDs)
		if err != nil {
			fmt.Printf("Error creating entertainment area: %v\n", err)
			return
		}

		// Save local configuration
		area := EntertainmentArea{
			ID:     groupID,
			Name:   areaName,
			Type:   "entertainment",
			Lights: validLightIDs,
		}

		if err := saveEntertainmentArea(area); err != nil {
			fmt.Printf("Warning: Failed to save local config: %v\n", err)
		}

		fmt.Printf("Entertainment area '%s' created (ID: %s) with %d lights\n", areaName, groupID, len(validLightIDs))
		fmt.Println("Use 'hue entertain stream start' to begin streaming")
	},
}

var entertainAreaDeleteCmd = &cobra.Command{
	Use:   "delete [area-name-or-id]",
	Short: "Delete an entertainment area",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		areaIdentifier := args[0]

		// Find area
		config, err := loadEntertainmentConfig()
		if err != nil {
			fmt.Printf("Error loading configuration: %v\n", err)
			return
		}

		var areaID string
		for _, area := range config.Areas {
			if area.Name == areaIdentifier || area.ID == areaIdentifier {
				areaID = area.ID
				break
			}
		}

		if areaID == "" {
			fmt.Printf("Entertainment area '%s' not found\n", areaIdentifier)
			return
		}

		// Delete from bridge
		if err := deleteEntertainmentGroup(areaID); err != nil {
			fmt.Printf("Error deleting from bridge: %v\n", err)
			return
		}

		// Remove from local config
		if err := removeEntertainmentArea(areaIdentifier); err != nil {
			fmt.Printf("Warning: Failed to update local config: %v\n", err)
		}

		fmt.Printf("Entertainment area deleted\n")
	},
}

var entertainAreaListCmd = &cobra.Command{
	Use:   "list",
	Short: "List entertainment areas",
	Run: func(cmd *cobra.Command, args []string) {
		// Fetch from bridge
		areas, err := getEntertainmentAreasFromBridge()
		if err != nil {
			fmt.Printf("Error fetching areas from bridge: %v\n", err)
			return
		}

		if len(areas) == 0 {
			fmt.Println("No entertainment areas configured on bridge")
			fmt.Println("Create one with: hue entertain area create [name] [light-ids...]")
			return
		}

		// Sync to local config
		config := &EntertainmentConfig{Areas: areas}
		if err := saveEntertainmentConfig(*config); err != nil {
			fmt.Printf("Warning: Failed to save local config: %v\n", err)
		}

		fmt.Println("Entertainment Areas:")
		for _, area := range areas {
			fmt.Printf("  %s (ID: %s)\n", area.Name, area.ID)
			fmt.Printf("    Lights: %s\n", strings.Join(area.Lights, ", "))
			if area.Stream != nil && area.Stream.Active {
				fmt.Printf("    Status: STREAMING (Owner: %s)\n", area.Stream.Owner)
			} else {
				fmt.Printf("    Status: Inactive\n")
			}
		}
	},
}

var entertainStreamCmd = &cobra.Command{
	Use:   "stream",
	Short: "Stream light updates",
	Long:  `Start streaming light updates to an entertainment area.`,
}

func init() {
	entertainStreamCmd.AddCommand(entertainStreamStartCmd)
	entertainStreamCmd.AddCommand(entertainStreamEffectCmd)
}

var entertainStreamStartCmd = &cobra.Command{
	Use:   "start [area-name-or-id]",
	Short: "Start streaming session",
	Long: `Start a streaming session to an entertainment area. This enables low-latency updates.
	
Note: Only one application can stream to an area at a time.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		areaIdentifier := args[0]

		// Try to find area locally first, then fetch from bridge
		config, err := loadEntertainmentConfig()
		if err != nil {
			config = &EntertainmentConfig{Areas: []EntertainmentArea{}}
		}

		var area *EntertainmentArea
		for i, a := range config.Areas {
			if a.Name == areaIdentifier || a.ID == areaIdentifier {
				area = &config.Areas[i]
				break
			}
		}

		// If not found locally, fetch from bridge
		if area == nil {
			areas, err := getEntertainmentAreasFromBridge()
			if err != nil {
				fmt.Printf("Error fetching areas from bridge: %v\n", err)
				return
			}

			for i, a := range areas {
				if a.Name == areaIdentifier || a.ID == areaIdentifier {
					area = &areas[i]
					break
				}
			}
		}

		if area == nil {
			fmt.Printf("Entertainment area '%s' not found\n", areaIdentifier)
			fmt.Println("Use 'hue entertain list' to see available areas")
			return
		}

		fmt.Printf("Starting streaming session for '%s'...\n", area.Name)

		// Activate streaming on the bridge
		if err := activateStreaming(area.ID); err != nil {
			fmt.Printf("Error activating streaming: %v\n", err)
			return
		}

		fmt.Printf("Streaming activated for area '%s'\n", area.Name)
		fmt.Println("\nStream is active. Use 'hue entertain stream effect' to send light data.")
		fmt.Println("Streaming will auto-deactivate after 10 seconds of inactivity.")
	},
}

var entertainStreamEffectCmd = &cobra.Command{
	Use:   "effect [area-name-or-id] [effect-name]",
	Short: "Stream a demo effect",
	Long: `Stream a demonstration effect to an entertainment area.
	
Available effects:
  rainbow    - Cycle through rainbow colors
  pulse      - Pulsing white light
  wave       - Color wave across lights
  random     - Random color flashing
  
Examples:
  hue entertain stream effect "Gaming Setup" rainbow
  hue entertain stream effect "TV Backlight" wave`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		areaIdentifier := args[0]
		effectName := args[1]

		// Try to find area locally first, then fetch from bridge
		config, err := loadEntertainmentConfig()
		if err != nil {
			config = &EntertainmentConfig{Areas: []EntertainmentArea{}}
		}

		var area *EntertainmentArea
		for i, a := range config.Areas {
			if a.Name == areaIdentifier || a.ID == areaIdentifier {
				area = &config.Areas[i]
				break
			}
		}

		// If not found locally, fetch from bridge
		if area == nil {
			areas, err := getEntertainmentAreasFromBridge()
			if err != nil {
				fmt.Printf("Error fetching areas from bridge: %v\n", err)
				return
			}

			for i, a := range areas {
				if a.Name == areaIdentifier || a.ID == areaIdentifier {
					area = &areas[i]
					break
				}
			}
		}

		if area == nil {
			fmt.Printf("Entertainment area '%s' not found\n", areaIdentifier)
			fmt.Println("Use 'hue entertain list' to see available areas")
			return
		}

		duration, _ := cmd.Flags().GetInt("duration")

		fmt.Printf("Streaming '%s' effect to '%s' for %d seconds...\n", effectName, area.Name, duration)

		// Start streaming
		if err := streamEffect(area, effectName, duration); err != nil {
			fmt.Printf("Error streaming effect: %v\n", err)
			return
		}

		fmt.Println("Effect completed")
	},
}

func init() {
	entertainStreamEffectCmd.Flags().IntP("duration", "d", 10, "Effect duration in seconds")
}

var entertainListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all entertainment areas",
	Run: func(cmd *cobra.Command, args []string) {
		// Fetch from bridge
		areas, err := getEntertainmentAreasFromBridge()
		if err != nil {
			fmt.Printf("Error fetching areas from bridge: %v\n", err)
			return
		}

		if len(areas) == 0 {
			fmt.Println("No entertainment areas found on bridge")
			fmt.Println("Create one with: hue entertain area create [name] [light-ids...]")
			return
		}

		// Sync to local config
		config := &EntertainmentConfig{Areas: areas}
		if err := saveEntertainmentConfig(*config); err != nil {
			fmt.Printf("Warning: Failed to save local config: %v\n", err)
		}

		fmt.Println("Entertainment areas:")
		for _, area := range areas {
			fmt.Printf("  %s (ID: %s, Lights: %d)\n", area.Name, area.ID, len(area.Lights))
			if area.Stream != nil && area.Stream.Active {
				fmt.Printf("    Status: STREAMING (Owner: %s)\n", area.Stream.Owner)
			}
		}
	},
}

// Helper functions

// buildBridgeURL constructs a URL for the bridge API, handling http:// prefix
func buildBridgeURL(host, path string) string {
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		return fmt.Sprintf("%s%s", host, path)
	}
	return fmt.Sprintf("http://%s%s", host, path)
}

func createEntertainmentGroup(name string, lightIDs []string) (string, error) {
	bridgeConfig, err := loadBridgeConfig()
	if err != nil {
		return "", err
	}

	// Create group payload
	payload := map[string]interface{}{
		"name":   name,
		"type":   "Entertainment",
		"lights": lightIDs,
		"class":  "TV", // Default class
	}

	data, _ := json.Marshal(payload)
	url := buildBridgeURL(bridgeConfig.Host, fmt.Sprintf("/api/%s/groups", bridgeConfig.Username))

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result []map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if len(result) > 0 {
		if success, ok := result[0]["success"].(map[string]interface{}); ok {
			if id, ok := success["id"].(string); ok {
				return id, nil
			}
		}
		if errMsg, ok := result[0]["error"].(map[string]interface{}); ok {
			return "", fmt.Errorf("%v", errMsg["description"])
		}
	}

	return "", fmt.Errorf("unexpected response: %s", string(body))
}

func deleteEntertainmentGroup(groupID string) error {
	bridgeConfig, err := loadBridgeConfig()
	if err != nil {
		return err
	}

	url := buildBridgeURL(bridgeConfig.Host, fmt.Sprintf("/api/%s/groups/%s", bridgeConfig.Username, groupID))
	req, _ := http.NewRequest("DELETE", url, nil)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func activateStreaming(groupID string) error {
	bridgeConfig, err := loadBridgeConfig()
	if err != nil {
		return err
	}

	payload := map[string]interface{}{
		"stream": map[string]bool{
			"active": true,
		},
	}

	data, _ := json.Marshal(payload)
	url := buildBridgeURL(bridgeConfig.Host, fmt.Sprintf("/api/%s/groups/%s", bridgeConfig.Username, groupID))

	req, _ := http.NewRequest("PUT", url, bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result []map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return err
	}

	// Check for errors in response
	if len(result) > 0 {
		if errMsg, ok := result[0]["error"].(map[string]interface{}); ok {
			errorType, _ := errMsg["type"].(float64)
			description := errMsg["description"]

			// Error type 1 = unauthorized user - needs link button press for streaming
			if errorType == 1 {
				fmt.Println("⚠️  Streaming requires additional authorization")
				fmt.Println("\nTo enable streaming:")
				fmt.Println("1. Press the link button on your Hue bridge")
				fmt.Println("2. Run this command again within 30 seconds")
				fmt.Println("\nNote: Standard API access is different from streaming API access.")
				fmt.Println("This is a one-time setup for Entertainment streaming.")
				return fmt.Errorf("unauthorized - link button press required")
			}
			return fmt.Errorf("%v", description)
		}
	}

	return nil
}

func getEntertainmentAreasFromBridge() ([]EntertainmentArea, error) {
	bridgeConfig, err := loadBridgeConfig()
	if err != nil {
		return nil, err
	}

	// Get all groups from the bridge
	url := buildBridgeURL(bridgeConfig.Host, fmt.Sprintf("/api/%s/groups", bridgeConfig.Username))
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get groups: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var groups map[string]interface{}
	if err := json.Unmarshal(body, &groups); err != nil {
		return nil, err
	}

	var areas []EntertainmentArea
	for groupID, groupData := range groups {
		groupMap, ok := groupData.(map[string]interface{})
		if !ok {
			continue
		}

		// Check if this is an entertainment group
		groupType, _ := groupMap["type"].(string)
		if groupType != "Entertainment" {
			continue
		}

		// Extract group information
		name, _ := groupMap["name"].(string)

		// Extract lights array
		var lightIDs []string
		if lightsData, ok := groupMap["lights"].([]interface{}); ok {
			for _, lightID := range lightsData {
				if id, ok := lightID.(string); ok {
					lightIDs = append(lightIDs, id)
				}
			}
		}

		// Extract stream configuration if present
		var streamConfig *StreamConfig
		if streamData, ok := groupMap["stream"].(map[string]interface{}); ok {
			active, _ := streamData["active"].(bool)
			owner, _ := streamData["owner"].(string)
			proxyMode, _ := streamData["proxymode"].(string)

			streamConfig = &StreamConfig{
				Active:    active,
				Owner:     owner,
				ProxyMode: proxyMode,
			}
		}

		area := EntertainmentArea{
			ID:     groupID,
			Name:   name,
			Type:   "entertainment",
			Lights: lightIDs,
			Stream: streamConfig,
		}

		areas = append(areas, area)
	}

	return areas, nil
}

func deactivateStreaming(groupID string) error {
	bridgeConfig, err := loadBridgeConfig()
	if err != nil {
		return err
	}

	payload := map[string]interface{}{
		"stream": map[string]bool{
			"active": false,
		},
	}

	data, _ := json.Marshal(payload)
	url := buildBridgeURL(bridgeConfig.Host, fmt.Sprintf("/api/%s/groups/%s", bridgeConfig.Username, groupID))

	req, _ := http.NewRequest("PUT", url, bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func streamEffect(area *EntertainmentArea, effectName string, durationSec int) error {
	// Activate streaming first
	if err := activateStreaming(area.ID); err != nil {
		return err
	}
	defer deactivateStreaming(area.ID)

	// Get entertainment key (clientkey) from bridge
	bridgeConfig, err := loadBridgeConfig()
	if err != nil {
		return err
	}

	// Get the entertainment configuration which includes the clientkey
	url := buildBridgeURL(bridgeConfig.Host, fmt.Sprintf("/api/%s/config", bridgeConfig.Username))
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to get bridge config: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var config map[string]interface{}
	if err := json.Unmarshal(body, &config); err != nil {
		return err
	}

	// Note: For full DTLS implementation, you would need the clientkey from the config
	// For now, we'll use HTTP updates as a fallback demonstration
	fmt.Println("Note: Using HTTP API for effects. Full DTLS streaming requires additional setup.")

	// Run the effect
	switch effectName {
	case "rainbow":
		return runRainbowEffect(area, durationSec)
	case "pulse":
		return runPulseEffect(area, durationSec)
	case "wave":
		return runWaveEffect(area, durationSec)
	case "random":
		return runRandomEffect(area, durationSec)
	default:
		return fmt.Errorf("unknown effect: %s", effectName)
	}
}

func runRainbowEffect(area *EntertainmentArea, durationSec int) error {
	lights, err := bridge.GetLights()
	if err != nil {
		return err
	}

	// Filter to area lights
	var areaLights []huego.Light
	for _, light := range lights {
		for _, idStr := range area.Lights {
			if fmt.Sprintf("%d", light.ID) == idStr {
				areaLights = append(areaLights, light)
				break
			}
		}
	}

	startTime := time.Now()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	hue := 0
	for time.Since(startTime) < time.Duration(durationSec)*time.Second {
		<-ticker.C

		for _, light := range areaLights {
			light.Hue(uint16(hue))
			light.Sat(254)
			light.Bri(254)
		}

		hue = (hue + 500) % 65535
	}

	return nil
}

func runPulseEffect(area *EntertainmentArea, durationSec int) error {
	lights, err := bridge.GetLights()
	if err != nil {
		return err
	}

	var areaLights []huego.Light
	for _, light := range lights {
		for _, idStr := range area.Lights {
			if fmt.Sprintf("%d", light.ID) == idStr {
				areaLights = append(areaLights, light)
				break
			}
		}
	}

	startTime := time.Now()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	step := 0
	for time.Since(startTime) < time.Duration(durationSec)*time.Second {
		<-ticker.C

		brightness := uint8((1.0 + float64(step%100)/50.0) * 127)
		if step%100 >= 50 {
			brightness = uint8((1.0 + float64(100-step%100)/50.0) * 127)
		}

		for _, light := range areaLights {
			light.Bri(brightness)
		}

		step++
	}

	return nil
}

func runWaveEffect(area *EntertainmentArea, durationSec int) error {
	lights, err := bridge.GetLights()
	if err != nil {
		return err
	}

	var areaLights []huego.Light
	for _, light := range lights {
		for _, idStr := range area.Lights {
			if fmt.Sprintf("%d", light.ID) == idStr {
				areaLights = append(areaLights, light)
				break
			}
		}
	}

	startTime := time.Now()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	step := 0
	for time.Since(startTime) < time.Duration(durationSec)*time.Second {
		<-ticker.C

		for i, light := range areaLights {
			hue := uint16((step*1000 + i*10000) % 65535)
			light.Hue(hue)
			light.Sat(254)
			light.Bri(254)
		}

		step++
	}

	return nil
}

func runRandomEffect(area *EntertainmentArea, durationSec int) error {
	lights, err := bridge.GetLights()
	if err != nil {
		return err
	}

	var areaLights []huego.Light
	for _, light := range lights {
		for _, idStr := range area.Lights {
			if fmt.Sprintf("%d", light.ID) == idStr {
				areaLights = append(areaLights, light)
				break
			}
		}
	}

	startTime := time.Now()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for time.Since(startTime) < time.Duration(durationSec)*time.Second {
		<-ticker.C

		for _, light := range areaLights {
			hue := uint16(time.Now().UnixNano() % 65535)
			light.Hue(hue)
			light.Sat(254)
			light.Bri(254)
		}
	}

	return nil
}

// DTLS Streaming (Advanced Implementation)

// EntertainmentStream represents a DTLS streaming connection
type EntertainmentStream struct {
	conn     *tls.Conn
	lights   []string
	isActive bool
}

// Note: Full DTLS implementation requires:
// 1. PSK-TLS cipher suite support
// 2. Entertainment key (clientkey) from bridge
// 3. DTLS over UDP (not standard TLS)
// 4. Binary protocol encoding

func createDTLSStream(area *EntertainmentArea, clientKey string) (*EntertainmentStream, error) {
	bridgeConfig, err := loadBridgeConfig()
	if err != nil {
		return nil, err
	}

	// DTLS PSK configuration
	// Note: This is a simplified example. Full implementation requires:
	// - PSK cipher suite (TLS_PSK_WITH_AES_128_GCM_SHA256)
	// - UDP transport instead of TCP
	// - Proper PSK callback configuration

	config := &tls.Config{
		InsecureSkipVerify: true, // In production, verify the bridge certificate
		// PSK configuration would go here
	}

	// Entertainment streaming uses port 2100 with DTLS
	addr := fmt.Sprintf("%s:2100", bridgeConfig.Host)

	// Note: Standard tls.Dial uses TCP, but Hue Entertainment requires UDP+DTLS
	// A full implementation would need a DTLS library like github.com/pion/dtls
	conn, err := tls.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("DTLS connection failed: %v (Note: Full DTLS/UDP implementation required)", err)
	}

	stream := &EntertainmentStream{
		conn:     conn,
		lights:   area.Lights,
		isActive: true,
	}

	return stream, nil
}

// SendLightStates sends light color data via DTLS streaming
func (s *EntertainmentStream) SendLightStates(lightStates map[string]RGB) error {
	if !s.isActive {
		return fmt.Errorf("stream not active")
	}

	// Entertainment protocol message format:
	// Header: "HueStream" (9 bytes)
	// Version: 0x01 0x00 (2 bytes)
	// Sequence: uint8 (1 byte)
	// Reserved: 0x00 0x00 (2 bytes)
	// Color space: 0x00 (RGB), 0x01 (XY+Brightness) (1 byte)
	// Reserved: 0x00 (1 byte)
	// Then for each light:
	//   Light ID: uint8 (1 byte) - 0-based index
	//   RGB: uint16 R, uint16 G, uint16 B (6 bytes per light)

	buf := new(bytes.Buffer)

	// Header
	buf.WriteString("HueStream")

	// Version
	binary.Write(buf, binary.BigEndian, uint16(0x0100))

	// Sequence number (increments each message)
	binary.Write(buf, binary.BigEndian, uint8(0))

	// Reserved
	binary.Write(buf, binary.BigEndian, uint16(0))

	// Color space (0 = RGB)
	buf.WriteByte(0x00)

	// Reserved
	buf.WriteByte(0x00)

	// Light data
	for i, lightID := range s.lights {
		if rgb, ok := lightStates[lightID]; ok {
			buf.WriteByte(uint8(i))
			binary.Write(buf, binary.BigEndian, uint16(rgb.R)<<8)
			binary.Write(buf, binary.BigEndian, uint16(rgb.G)<<8)
			binary.Write(buf, binary.BigEndian, uint16(rgb.B)<<8)
		}
	}

	_, err := s.conn.Write(buf.Bytes())
	return err
}

type RGB struct {
	R, G, B uint8
}

func (s *EntertainmentStream) Close() error {
	s.isActive = false
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

// Configuration file helpers

func loadEntertainmentConfig() (*EntertainmentConfig, error) {
	var config EntertainmentConfig
	data, err := os.ReadFile(entertainmentFile)
	if err != nil {
		return &EntertainmentConfig{Areas: []EntertainmentArea{}}, nil
	}
	err = json.Unmarshal(data, &config)
	return &config, err
}

func saveEntertainmentConfig(config EntertainmentConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(entertainmentFile, data, 0600)
}

func saveEntertainmentArea(area EntertainmentArea) error {
	config, err := loadEntertainmentConfig()
	if err != nil {
		return err
	}

	// Update or add area
	found := false
	for i, a := range config.Areas {
		if a.ID == area.ID || a.Name == area.Name {
			config.Areas[i] = area
			found = true
			break
		}
	}

	if !found {
		config.Areas = append(config.Areas, area)
	}

	return saveEntertainmentConfig(*config)
}

func removeEntertainmentArea(identifier string) error {
	config, err := loadEntertainmentConfig()
	if err != nil {
		return err
	}

	// Find and remove area
	for i, area := range config.Areas {
		if area.Name == identifier || area.ID == identifier {
			config.Areas = append(config.Areas[:i], config.Areas[i+1:]...)
			return saveEntertainmentConfig(*config)
		}
	}

	return fmt.Errorf("area not found")
}
