package main

import (
	"bytes"
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
	// Deactivate any existing streaming session first
	deactivateStreaming(area.ID)
	time.Sleep(500 * time.Millisecond) // Give bridge time to clean up

	// Activate streaming
	if err := activateStreaming(area.ID); err != nil {
		return err
	}
	defer deactivateStreaming(area.ID)

	// Try DTLS streaming first
	dtlsStream, err := createDTLSStreamConnection(area)
	if err != nil {
		// Fall back to HTTP if DTLS fails
		fmt.Printf("DTLS connection failed (%v), falling back to HTTP API\n", err)
		return streamEffectHTTP(area, effectName, durationSec)
	}
	defer dtlsStream.Close()

	fmt.Println("✅ DTLS streaming connected - High-speed mode active (~60fps)")

	// Stream the effect using DTLS
	return dtlsStream.StreamEffect(effectName, durationSec)
}

// streamEffectHTTP is the fallback HTTP implementation
func streamEffectHTTP(area *EntertainmentArea, effectName string, durationSec int) error {
	fmt.Println("Note: Using HTTP API for effects (~10 updates/sec)")

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
