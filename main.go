package main

import (
  "encoding/json"
  "fmt"
  "math"
  "os"
  "path/filepath"
  "regexp"
  "strconv"
  "strings"
  "time"

  "github.com/amimof/huego"
  "github.com/spf13/cobra"
)

type BridgeConfig struct {
  Host     string `json:"host"`
  Username string `json:"username"`
  ID       string `json:"id"`
}

type SceneCommand struct {
  Type   string   `json:"type"`   // "on", "off", "brightness", "color"
  Light  string   `json:"light"`  // light name or ID
  Values []string `json:"values"` // command arguments
}

type Scene struct {
  Name     string         `json:"name"`
  Commands []SceneCommand `json:"commands"`
}

type SceneConfig struct {
  Scenes []Scene `json:"scenes"`
}

type Group struct {
  Name   string   `json:"name"`
  Lights []string `json:"lights"` // light names or IDs
}

type GroupConfig struct {
  Groups []Group `json:"groups"`
}

var bridge *huego.Bridge
var configFile string
var sceneFile string
var groupFile string

func main() {
  // Set config file path
  homeDir, err := os.UserHomeDir()
  if err != nil {
    fmt.Printf("Error getting home directory: %v\n", err)
    os.Exit(1)
  }
  configFile = filepath.Join(homeDir, ".hue-config.json")
  sceneFile = filepath.Join(homeDir, ".hue-scenes.json")
  groupFile = filepath.Join(homeDir, ".hue-groups.json")

  var rootCmd = &cobra.Command{
    Use:   "hue",
    Short: "A CLI tool for controlling Philips Hue lights",
    Long:  `A command line interface for discovering and controlling Philips Hue lights in your network.`,
    PersistentPreRun: func(cmd *cobra.Command, args []string) {
      // Skip bridge initialization for auth, status, find, discover, scene management, and group management commands
      cmdName := cmd.Name()
      parentCmdName := cmd.Parent().Name()
      skipInit := cmdName == "auth" || cmdName == "discover" || cmdName == "status" ||
        cmdName == "find" || cmdName == "scenes" || cmdName == "groups" ||
        (cmdName == "list" && (parentCmdName == "scene" || parentCmdName == "group")) ||
        (cmdName == "remove" && (parentCmdName == "scene" || parentCmdName == "group"))

      if !skipInit {
        initBridge()
      }
    },
  }

  rootCmd.AddCommand(authCmd)
  rootCmd.AddCommand(statusCmd)
  rootCmd.AddCommand(findCmd)
  rootCmd.AddCommand(listCmd)
  rootCmd.AddCommand(onCmd)
  rootCmd.AddCommand(offCmd)
  rootCmd.AddCommand(brightnessCmd)
  rootCmd.AddCommand(colorCmd)
  rootCmd.AddCommand(discoverCmd)
  rootCmd.AddCommand(sceneCmd)
  rootCmd.AddCommand(groupCmd)

  if err := rootCmd.Execute(); err != nil {
    fmt.Println(err)
    os.Exit(1)
  }
}

func initBridge() {
  // Try to load saved configuration first
  config, err := loadBridgeConfig()
  if err == nil && config.Host != "" && config.Username != "" {
    // Use saved configuration
    bridge = huego.New(config.Host, config.Username)
    return
  }

  // If no saved config, try to discover and prompt for authorization
  fmt.Println("No saved bridge configuration found.")
  fmt.Println("Please run 'hue auth' to authorize with your Hue bridge first.")
  os.Exit(1)
}

var authCmd = &cobra.Command{
  Use:   "auth",
  Short: "Authorize with Hue bridge",
  Long:  `Discover and authorize with your Hue bridge. This will save the credentials for future use.`,
  Run: func(cmd *cobra.Command, args []string) {
    bridgeIP, _ := cmd.Flags().GetString("ip")

    var discoveredBridge *huego.Bridge
    var err error

    if bridgeIP != "" {
      fmt.Printf("Connecting to bridge at %s...\n", bridgeIP)
      discoveredBridge = huego.New(bridgeIP, "")

      // Test if the bridge is reachable
      _, err = discoveredBridge.GetConfig()
      if err != nil {
        fmt.Printf("Error connecting to bridge at %s: %v\n", bridgeIP, err)
        fmt.Println("Please check the IP address and try again.")
        return
      }
      // Set the ID to the IP for manual connections
      discoveredBridge.ID = bridgeIP
    } else {
      fmt.Println("Discovering Hue bridge...")
      fmt.Println("This may take a few seconds...")

      discoveredBridge, err = huego.Discover()
      if err != nil {
        fmt.Printf("Error discovering bridge: %v\n", err)
        fmt.Println("\nTroubleshooting steps:")
        fmt.Println("1. Make sure your computer and Hue bridge are on the same network")
        fmt.Println("2. Check that your Hue bridge is powered on and connected")
        fmt.Println("3. Try using the Philips Hue app to ensure the bridge is working")
        fmt.Println("4. You can also try specifying the bridge IP manually with:")
        fmt.Println("   hue auth --ip <bridge-ip-address>")
        return
      }

      if discoveredBridge == nil {
        fmt.Println("No Hue bridge found on the network.")
        fmt.Println("Please check that:")
        fmt.Println("1. Your Hue bridge is powered on")
        fmt.Println("2. Your computer and bridge are on the same network")
        fmt.Println("3. No firewall is blocking the discovery")
        fmt.Println("4. Try specifying the bridge IP manually: hue auth --ip <bridge-ip>")
        return
      }
    }

    fmt.Printf("Found bridge at: %s\n", discoveredBridge.Host)
    fmt.Printf("Bridge ID: %s\n", discoveredBridge.ID)
    fmt.Println("\nPress the link button on your Hue bridge now...")
    fmt.Println("You have 30 seconds to press the button.")
    fmt.Println("Waiting for authorization...")

    // Try to create a new user every 2 seconds for 30 seconds
    var username string
    for i := 0; i < 15; i++ {
      time.Sleep(2 * time.Second)
      username, err = discoveredBridge.CreateUser("hue")
      if err == nil {
        break
      }
      fmt.Print(".")
    }

    if err != nil {
      fmt.Printf("\nFailed to authorize: %v\n", err)
      fmt.Println("Make sure you pressed the link button on your bridge.")
      fmt.Println("If the problem persists, try:")
      fmt.Println("1. Wait a minute and try again")
      fmt.Println("2. Check that no other apps are trying to connect")
      return
    }

    fmt.Printf("\nSuccess! Authorized with username: %s\n", username)

    // Save configuration
    config := BridgeConfig{
      Host:     discoveredBridge.Host,
      Username: username,
      ID:       discoveredBridge.ID,
    }

    err = saveBridgeConfig(config)
    if err != nil {
      fmt.Printf("Warning: Failed to save configuration: %v\n", err)
      fmt.Println("You may need to authorize again next time.")
    } else {
      fmt.Printf("Configuration saved to: %s\n", configFile)
      fmt.Println("You can now use other hue commands!")
    }
  },
}

func init() {
  authCmd.Flags().StringP("ip", "i", "", "Manually specify bridge IP address instead of auto-discovery")
}

var statusCmd = &cobra.Command{
  Use:   "status",
  Short: "Show authorization status",
  Long:  `Display the current authorization status and bridge information.`,
  Run: func(cmd *cobra.Command, args []string) {
    config, err := loadBridgeConfig()
    if err != nil {
      fmt.Println("Status: Not authorized")
      fmt.Printf("Config file: %s (not found)\n", configFile)
      fmt.Println("Run 'hue auth' to authorize with your Hue bridge.")
      return
    }

    fmt.Println("Status: Authorized")
    fmt.Printf("Config file: %s\n", configFile)
    fmt.Printf("Bridge Host: %s\n", config.Host)
    fmt.Printf("Bridge ID: %s\n", config.ID)
    fmt.Printf("Username: %s\n", config.Username)

    // Test connection
    testBridge := huego.New(config.Host, config.Username)
    if lights, err := testBridge.GetLights(); err == nil {
      fmt.Printf("Connection: OK (%d lights found)\n", len(lights))
    } else {
      fmt.Printf("Connection: FAILED (%v)\n", err)
      fmt.Println("You may need to re-authorize with 'hue auth'")
    }
  },
}

var findCmd = &cobra.Command{
  Use:   "find",
  Short: "Find Hue bridge on network",
  Long:  `Attempt to find your Hue bridge using multiple methods and show diagnostic information.`,
  Run: func(cmd *cobra.Command, args []string) {
    fmt.Println("Searching for Hue bridge...")
    fmt.Println("Method 1: Using Philips discovery service...")

    bridge, err := huego.Discover()
    if err != nil {
      fmt.Printf("Discovery service failed: %v\n", err)
    } else if bridge != nil {
      fmt.Printf("✓ Found bridge at: %s (ID: %s)\n", bridge.Host, bridge.ID)

      // Test connection
      if config, err := bridge.GetConfig(); err == nil {
        fmt.Printf("  Bridge name: %s\n", config.Name)
        fmt.Printf("  API version: %s\n", config.APIVersion)
        fmt.Printf("  Software version: %s\n", config.SwVersion)
      }
    } else {
      fmt.Println("No bridge found via discovery service")
    }

    fmt.Println("\nMethod 2: Checking common IP ranges...")
    fmt.Println("Scanning local network for Hue bridges...")

    // Get local IP to determine network range
    fmt.Println("To manually find your bridge IP:")
    fmt.Println("1. Open your router's admin panel (usually 192.168.1.1 or 192.168.0.1)")
    fmt.Println("2. Look for connected devices named 'Philips-hue' or similar")
    fmt.Println("3. Use that IP address with: hue auth --ip <ip-address>")
    fmt.Println("\nAlternatively, try the Philips Hue app to ensure your bridge is working properly.")
  },
}

var listCmd = &cobra.Command{
  Use:   "list",
  Short: "List all Hue lights",
  Long:  `Display a list of all Philips Hue lights with their current status.`,
  Run: func(cmd *cobra.Command, args []string) {
    lights, err := bridge.GetLights()
    if err != nil {
      fmt.Printf("Error getting lights: %v\n", err)
      return
    }

    fmt.Println("Hue Lights:")
    fmt.Println("ID\tName\t\t\tOn\tBrightness\tHue\tSaturation")
    fmt.Println("--\t----\t\t\t--\t----------\t---\t----------")
    for _, light := range lights {
      fmt.Printf("%d\t%-20s\t%t\t%d\t\t%d\t%d\n",
        light.ID, light.Name, light.State.On, light.State.Bri, light.State.Hue, light.State.Sat)
    }
  },
}

var onCmd = &cobra.Command{
  Use:   "on [light-id/light-name/group]",
  Short: "Turn on lights",
  Long:  `Turn on one or more lights by ID, name, or group. Use 'all' to turn on all lights. Use 'g:groupname' to turn on a group.`,
  Args:  cobra.MinimumNArgs(1),
  Run: func(cmd *cobra.Command, args []string) {
    if args[0] == "all" {
      lights, err := bridge.GetLights()
      if err != nil {
        fmt.Printf("Error getting lights: %v\n", err)
        return
      }
      for _, light := range lights {
        light.On()
      }
      fmt.Println("All lights turned on")
      return
    }

    lights := resolveLightIdentifiers(args)
    if len(lights) == 0 {
      fmt.Printf("No lights found for identifiers: %v\n", args)
      return
    }

    for _, light := range lights {
      light.On()
      fmt.Printf("Light '%s' turned on\n", light.Name)
    }

    if len(lights) > 1 {
      fmt.Printf("Total: %d lights turned on\n", len(lights))
    }
  },
}

var offCmd = &cobra.Command{
  Use:   "off [light-id/light-name/group]",
  Short: "Turn off lights",
  Long:  `Turn off one or more lights by ID, name, or group. Use 'all' to turn off all lights. Use 'g:groupname' to turn off a group.`,
  Args:  cobra.MinimumNArgs(1),
  Run: func(cmd *cobra.Command, args []string) {
    if args[0] == "all" {
      lights, err := bridge.GetLights()
      if err != nil {
        fmt.Printf("Error getting lights: %v\n", err)
        return
      }
      for _, light := range lights {
        light.Off()
      }
      fmt.Println("All lights turned off")
      return
    }

    lights := resolveLightIdentifiers(args)
    if len(lights) == 0 {
      fmt.Printf("No lights found for identifiers: %v\n", args)
      return
    }

    for _, light := range lights {
      light.Off()
      fmt.Printf("Light '%s' turned off\n", light.Name)
    }

    if len(lights) > 1 {
      fmt.Printf("Total: %d lights turned off\n", len(lights))
    }
  },
}

var brightnessCmd = &cobra.Command{
  Use:   "brightness [light-id/light-name/group] [0-254]",
  Short: "Set brightness of lights",
  Long:  `Set the brightness of one or more lights. Brightness range is 0-254. Use 'g:groupname' to set brightness for a group.`,
  Args:  cobra.ExactArgs(2),
  Run: func(cmd *cobra.Command, args []string) {
    brightness, err := strconv.Atoi(args[1])
    if err != nil || brightness < 0 || brightness > 254 {
      fmt.Println("Brightness must be a number between 0 and 254")
      return
    }

    if args[0] == "all" {
      lights, err := bridge.GetLights()
      if err != nil {
        fmt.Printf("Error getting lights: %v\n", err)
        return
      }
      for _, light := range lights {
        light.Bri(uint8(brightness))
      }
      fmt.Printf("All lights brightness set to %d\n", brightness)
      return
    }

    lights := resolveLightIdentifiers([]string{args[0]})
    if len(lights) == 0 {
      fmt.Printf("No lights found for identifier: %s\n", args[0])
      return
    }

    for _, light := range lights {
      light.Bri(uint8(brightness))
      fmt.Printf("Light '%s' brightness set to %d\n", light.Name, brightness)
    }

    if len(lights) > 1 {
      fmt.Printf("Total: %d lights brightness set to %d\n", len(lights), brightness)
    }
  },
}

var colorCmd = &cobra.Command{
  Use:   "color [light-id/light-name/group] [red] [green] [blue] [brightness] OR [light-id/light-name/group] [RRGGBB|RRGGBBAA|RGB|RGBA]",
  Short: "Set RGB color of lights",
  Long:  `Set the RGB color of one or more lights. RGB values should be between 0-255. Brightness is optional (0-254). You can also use Hex color codes (RRGGBB, RRGGBBAA, RGB, RGBA). Use 'g:groupname' to set color for a group.`,
  Args:  cobra.RangeArgs(1, 5),
  Run: func(cmd *cobra.Command, args []string) {
    if len(args) == 2 {
      // Check if args[1] is a hex color code using regex
      hex := args[1]
      if (len(hex) > 2) && isHexColor(hex) {
        r, g, b, a := parseHexColor(hex)
        args[1] = fmt.Sprintf("%d", r)
        args = append(args, fmt.Sprintf("%d", g))
        args = append(args, fmt.Sprintf("%d", b))
        if a >= 0 {
          args = append(args, fmt.Sprintf("%d", a))
        }
      } else {
        // If not a hex code, we expect RGB values
        if len(args) < 4 {
          fmt.Println("RGB values must be provided")
          return
        }
      }
    }

    r, err1 := strconv.Atoi(args[1])
    g, err2 := strconv.Atoi(args[2])
    b, err3 := strconv.Atoi(args[3])

    if err1 != nil || err2 != nil || err3 != nil ||
      r < 0 || r > 255 || g < 0 || g > 255 || b < 0 || b > 255 {
      fmt.Println("RGB values must be numbers between 0 and 255")
      return
    }

    // Check for optional brightness parameter
    var brightness int = -1 // -1 means no brightness set
    if len(args) == 5 {
      var err4 error
      brightness, err4 = strconv.Atoi(args[4])
      if err4 != nil || brightness < 0 || brightness > 255 {
        if brightness > 254 {
          brightness = 254 // Just to not break the user script if they provide 255 or FF as that feels more natural
        }
        fmt.Println("Brightness value must be a number between 0 and 254")
        return
      }
    }

    if args[0] == "all" {
      lights, err := bridge.GetLights()
      if err != nil {
        fmt.Printf("Error getting lights: %v\n", err)
        return
      }
      for _, light := range lights {
        // Convert RGB to XY color space for Hue lights
        x, y := rgbToXY(uint8(r), uint8(g), uint8(b))
        light.Xy([]float32{x, y})
        if brightness >= 0 {
          light.Bri(uint8(brightness))
        }
      }
      if brightness >= 0 {
        fmt.Printf("All lights color set to RGB(%d, %d, %d) with brightness %d\n", r, g, b, brightness)
      } else {
        fmt.Printf("All lights color set to RGB(%d, %d, %d)\n", r, g, b)
      }
      return
    }

    lights := resolveLightIdentifiers([]string{args[0]})
    if len(lights) == 0 {
      fmt.Printf("No lights found for identifier: %s\n", args[0])
      return
    }

    for _, light := range lights {
      // Convert RGB to XY color space for Hue lights
      x, y := rgbToXY(uint8(r), uint8(g), uint8(b))
      light.Xy([]float32{x, y})
      if brightness >= 0 {
        light.Bri(uint8(brightness))
        fmt.Printf("Light '%s' color set to RGB(%d, %d, %d) with brightness %d\n", light.Name, r, g, b, brightness)
      } else {
        fmt.Printf("Light '%s' color set to RGB(%d, %d, %d)\n", light.Name, r, g, b)
      }
    }

    if len(lights) > 1 {
      if brightness >= 0 {
        fmt.Printf("Total: %d lights color set to RGB(%d, %d, %d) with brightness %d\n", len(lights), r, g, b, brightness)
      } else {
        fmt.Printf("Total: %d lights color set to RGB(%d, %d, %d)\n", len(lights), r, g, b)
      }
    }
  },
}

func parseHexColor(s string) (r, g, b, a int) {
  if len(s) == 6 || len(s) == 8 {
    r64, err := strconv.ParseUint(s[0:2], 16, 8)
    if err != nil {
      return 0, 0, 0, -1
    }
    g64, err := strconv.ParseUint(s[2:4], 16, 8)
    if err != nil {
      return 0, 0, 0, -1
    }
    b64, err := strconv.ParseUint(s[4:6], 16, 8)
    if err != nil {
      return 0, 0, 0, -1
    }
    if len(s) == 8 {
      a64, err := strconv.ParseUint(s[6:8], 16, 8)
      if err != nil {
        return 0, 0, 0, -1
      }
      return int(r64), int(g64), int(b64), int(a64)
    }
    return int(r64), int(g64), int(b64), -1
  } else if len(s) == 3 || len(s) == 4 {
    r64, err := strconv.ParseUint(string(s[0])+string(s[0]), 16, 8)
    if err != nil {
      return 0, 0, 0, -1
    }
    g64, err := strconv.ParseUint(string(s[1])+string(s[1]), 16, 8)
    if err != nil {
      return 0, 0, 0, -1
    }
    b64, err := strconv.ParseUint(string(s[2])+string(s[2]), 16, 8)
    if err != nil {
      return 0, 0, 0, -1
    }
    if len(s) == 4 {
      a64, err := strconv.ParseUint(string(s[3])+string(s[3]), 16, 8)
      if err != nil {
        return 0, 0, 0, -1
      }
      return int(r64), int(g64), int(b64), int(a64)
    }
    return int(r64), int(g64), int(b64), -1
  }
  return 0, 0, 0, -1
}

func isHexColor(s string) bool {
  matched, _ := regexp.MatchString(`^[0-9a-fA-F]{6}|^[0-9a-fA-F]{3}$`, s)
  return matched
}

var discoverCmd = &cobra.Command{
  Use:   "discover",
  Short: "Discover Hue bridge in network",
  Long:  `Discover and display information about the Hue bridge in your network.`,
  Run: func(cmd *cobra.Command, args []string) {
    bridge, err := huego.Discover()
    if err != nil {
      fmt.Printf("Error discovering bridge: %v\n", err)
      return
    }

    fmt.Printf("Bridge found at: %s\n", bridge.Host)
    fmt.Printf("Bridge ID: %s\n", bridge.ID)

    // Try to get bridge info
    if info, err := bridge.GetConfig(); err == nil {
      fmt.Printf("Bridge Name: %s\n", info.Name)
      fmt.Printf("API Version: %s\n", info.APIVersion)
      fmt.Printf("Software Version: %s\n", info.SwVersion)
    }
  },
}

func findLight(identifier string) *huego.Light {
  lights := resolveLightIdentifiers([]string{identifier})
  if len(lights) > 0 {
    return &lights[0]
  }
  return nil
}

// resolveLightIdentifiers resolves a list of identifiers (IDs, names, or groups) to actual lights
func resolveLightIdentifiers(identifiers []string) []huego.Light {
  allLights, err := bridge.GetLights()
  if err != nil {
    return nil
  }

  var resolvedLights []huego.Light
  seenLights := make(map[int]bool) // prevent duplicates

  for _, identifier := range identifiers {
    // Check if it's a group reference (starts with "g:")
    if strings.HasPrefix(identifier, "g:") {
      groupName := identifier[2:] // remove "g:" prefix
      groupLights := resolveGroup(groupName, allLights)
      for _, light := range groupLights {
        if !seenLights[light.ID] {
          resolvedLights = append(resolvedLights, light)
          seenLights[light.ID] = true
        }
      }
    } else {
      // Single light (ID or name)
      light := resolveSingleLight(identifier, allLights)
      if light != nil && !seenLights[light.ID] {
        resolvedLights = append(resolvedLights, *light)
        seenLights[light.ID] = true
      }
    }
  }

  return resolvedLights
}

func resolveSingleLight(identifier string, allLights []huego.Light) *huego.Light {
  // Try to find by ID first
  if id, err := strconv.Atoi(identifier); err == nil {
    for _, light := range allLights {
      if light.ID == id {
        return &light
      }
    }
  }

  // Try to find by name (case-insensitive partial match)
  identifier = strings.ToLower(identifier)
  for _, light := range allLights {
    if strings.Contains(strings.ToLower(light.Name), identifier) {
      return &light
    }
  }

  return nil
}

func resolveGroup(groupName string, allLights []huego.Light) []huego.Light {
  config, err := loadGroupConfig()
  if err != nil {
    return nil
  }

  // Find the group
  var group *Group
  for _, g := range config.Groups {
    if strings.EqualFold(g.Name, groupName) {
      group = &g
      break
    }
  }

  if group == nil {
    return nil
  }

  var groupLights []huego.Light
  for _, lightIdentifier := range group.Lights {
    light := resolveSingleLight(lightIdentifier, allLights)
    if light != nil {
      groupLights = append(groupLights, *light)
    }
  }

  return groupLights
}

// rgbToXY converts RGB values to XY color space for Hue lights
func rgbToXY(r, g, b uint8) (float32, float32) {
  // Normalize RGB values to 0-1
  red := float64(r) / 255.0
  green := float64(g) / 255.0
  blue := float64(b) / 255.0

  // Apply gamma correction
  red = gammaCorrect(red)
  green = gammaCorrect(green)
  blue = gammaCorrect(blue)

  // Convert to XYZ color space
  X := red*0.664511 + green*0.154324 + blue*0.162028
  Y := red*0.283881 + green*0.668433 + blue*0.047685
  Z := red*0.000088 + green*0.072310 + blue*0.986039

  // Convert XYZ to xy
  sum := X + Y + Z
  if sum == 0 {
    return 0.0, 0.0
  }

  x := float32(X / sum)
  y := float32(Y / sum)

  return x, y
}

func gammaCorrect(value float64) float64 {
  if value > 0.04045 {
    return math.Pow((value+0.055)/1.055, 2.4)
  }
  return value / 12.92
}

// saveBridgeConfig saves the bridge configuration to disk
func saveBridgeConfig(config BridgeConfig) error {
  data, err := json.MarshalIndent(config, "", "  ")
  if err != nil {
    return err
  }
  return os.WriteFile(configFile, data, 0600)
}

// loadBridgeConfig loads the bridge configuration from disk
func loadBridgeConfig() (BridgeConfig, error) {
  var config BridgeConfig
  data, err := os.ReadFile(configFile)
  if err != nil {
    return config, err
  }
  err = json.Unmarshal(data, &config)
  return config, err
}

// Scene commands
var sceneCmd = &cobra.Command{
  Use:   "scene",
  Short: "Manage and execute light scenes",
  Long:  `Create, manage, and execute custom light scenes. Scenes allow you to save and replay complex lighting configurations.`,
  Args:  cobra.MinimumNArgs(1),
  Run: func(cmd *cobra.Command, args []string) {
    if len(args) == 1 {
      // Execute scene
      executeScene(args[0])
    } else {
      cmd.Help()
    }
  },
}

func init() {
  // Add subcommands to scene
  sceneCmd.AddCommand(sceneAddCmd)
  sceneCmd.AddCommand(sceneListCmd)
  sceneCmd.AddCommand(sceneRemoveCmd)
  sceneCmd.AddCommand(scenesListAllCmd)
}

var sceneAddCmd = &cobra.Command{
  Use:   "add [scene-name] [command-type] [light/group] [args...]",
  Short: "Add a command to a scene",
  Long: `Add a light command to a scene. Commands will be executed concurrently when the scene is run.
  
Examples:
  hue scene add "movie-night" color "Living Room" 255 100 50
  hue scene add "movie-night" brightness "Bedroom" 50
  hue scene add "movie-night" on "Kitchen"
  hue scene add "movie-night" off "g:hallway"
  hue scene add "movie-night" color "g:living-room" 255 100 50`,
  Args: cobra.MinimumNArgs(4),
  Run: func(cmd *cobra.Command, args []string) {
    sceneName := args[0]
    commandType := args[1]
    lightOrGroup := args[2]
    values := args[3:]

    // Validate command type
    validTypes := map[string]bool{"on": true, "off": true, "brightness": true, "color": true}
    if !validTypes[commandType] {
      fmt.Printf("Invalid command type '%s'. Valid types: on, off, brightness, color\n", commandType)
      return
    }

    // Validate arguments based on command type
    if err := validateSceneCommand(commandType, values); err != nil {
      fmt.Printf("Error: %v\n", err)
      return
    }

    // Verify light or group exists
    if strings.HasPrefix(lightOrGroup, "g:") {
      // Verify group exists
      groupName := lightOrGroup[2:]
      config, err := loadGroupConfig()
      if err != nil || findGroup(config, groupName) == nil {
        fmt.Printf("Warning: Group '%s' not found. Command will be saved but may fail during execution.\n", groupName)
      }
    } else {
      // Verify light exists
      if findLight(lightOrGroup) == nil {
        fmt.Printf("Warning: Light '%s' not found. Command will be saved but may fail during execution.\n", lightOrGroup)
      }
    }

    // Add command to scene
    if err := addCommandToScene(sceneName, commandType, lightOrGroup, values); err != nil {
      fmt.Printf("Error adding command to scene: %v\n", err)
      return
    }

    fmt.Printf("Added %s command for '%s' to scene '%s'\n", commandType, lightOrGroup, sceneName)
  },
}

var sceneListCmd = &cobra.Command{
  Use:   "list [scene-name]",
  Short: "List commands in a scene",
  Long:  `Display all commands that will be executed when a scene is run.`,
  Args:  cobra.ExactArgs(1),
  Run: func(cmd *cobra.Command, args []string) {
    sceneName := args[0]

    config, err := loadSceneConfig()
    if err != nil {
      fmt.Printf("Error loading scenes: %v\n", err)
      return
    }

    scene := findScene(config, sceneName)
    if scene == nil {
      fmt.Printf("Scene '%s' not found\n", sceneName)
      return
    }

    fmt.Printf("Scene '%s' commands:\n", sceneName)
    for i, command := range scene.Commands {
      fmt.Printf("%d. %s \"%s\"", i+1, command.Type, command.Light)
      for _, value := range command.Values {
        fmt.Printf(" %s", value)
      }
      fmt.Println()
    }
  },
}

var sceneRemoveCmd = &cobra.Command{
  Use:   "remove [scene-name] [command-index]",
  Short: "Remove a command from a scene or remove entire scene",
  Long: `Remove a specific command from a scene by index, or remove the entire scene if no index is provided.
  
Examples:
  hue scene remove "movie-night" 1    # Remove command at index 1
  hue scene remove "movie-night"      # Remove entire scene`,
  Args: cobra.RangeArgs(1, 2),
  Run: func(cmd *cobra.Command, args []string) {
    sceneName := args[0]

    if len(args) == 1 {
      // Remove entire scene
      if err := removeScene(sceneName); err != nil {
        fmt.Printf("Error removing scene: %v\n", err)
        return
      }
      fmt.Printf("Scene '%s' removed\n", sceneName)
    } else {
      // Remove specific command
      index, err := strconv.Atoi(args[1])
      if err != nil {
        fmt.Printf("Invalid index: %s\n", args[1])
        return
      }

      if err := removeCommandFromScene(sceneName, index-1); err != nil {
        fmt.Printf("Error removing command: %v\n", err)
        return
      }
      fmt.Printf("Command %d removed from scene '%s'\n", index, sceneName)
    }
  },
}

var scenesListAllCmd = &cobra.Command{
  Use:   "scenes",
  Short: "List all available scenes",
  Long:  `Display all saved scenes and their command counts.`,
  Run: func(cmd *cobra.Command, args []string) {
    config, err := loadSceneConfig()
    if err != nil {
      fmt.Printf("Error loading scenes: %v\n", err)
      return
    }

    if len(config.Scenes) == 0 {
      fmt.Println("No scenes found")
      return
    }

    fmt.Println("Available scenes:")
    for _, scene := range config.Scenes {
      fmt.Printf("  %s (%d commands)\n", scene.Name, len(scene.Commands))
    }
  },
}

// Scene helper functions
func validateSceneCommand(commandType string, values []string) error {
  switch commandType {
  case "on", "off":
    if len(values) != 0 {
      return fmt.Errorf("%s command takes no additional arguments", commandType)
    }
  case "brightness":
    if len(values) != 1 {
      return fmt.Errorf("brightness command requires exactly 1 argument (0-254)")
    }
    brightness, err := strconv.Atoi(values[0])
    if err != nil || brightness < 0 || brightness > 254 {
      return fmt.Errorf("brightness must be a number between 0 and 254")
    }
  case "color":
    if len(values) != 3 && len(values) != 4 {
      return fmt.Errorf("color command requires 3 arguments (red green blue) or 4 arguments (red green blue brightness)")
    }
    for i, value := range values {
      if i < 3 {
        // RGB values
        colorValue, err := strconv.Atoi(value)
        if err != nil || colorValue < 0 || colorValue > 255 {
          colors := []string{"red", "green", "blue"}
          return fmt.Errorf("%s value must be a number between 0 and 255", colors[i])
        }
      } else {
        // Brightness value (4th parameter)
        brightness, err := strconv.Atoi(value)
        if err != nil || brightness < 0 || brightness > 254 {
          return fmt.Errorf("brightness value must be a number between 0 and 254")
        }
      }
    }
  }
  return nil
}

func addCommandToScene(sceneName, commandType, light string, values []string) error {
  config, err := loadSceneConfig()
  if err != nil {
    // Create new config if file doesn't exist
    config = &SceneConfig{Scenes: []Scene{}}
  }

  // Find or create scene
  scene := findScene(config, sceneName)
  if scene == nil {
    config.Scenes = append(config.Scenes, Scene{
      Name:     sceneName,
      Commands: []SceneCommand{},
    })
    scene = &config.Scenes[len(config.Scenes)-1]
  }

  // Add command to scene
  command := SceneCommand{
    Type:   commandType,
    Light:  light,
    Values: values,
  }
  scene.Commands = append(scene.Commands, command)

  return saveSceneConfig(*config)
}

func removeCommandFromScene(sceneName string, index int) error {
  config, err := loadSceneConfig()
  if err != nil {
    return err
  }

  scene := findScene(config, sceneName)
  if scene == nil {
    return fmt.Errorf("scene '%s' not found", sceneName)
  }

  if index < 0 || index >= len(scene.Commands) {
    return fmt.Errorf("invalid command index %d", index+1)
  }

  // Remove command at index
  scene.Commands = append(scene.Commands[:index], scene.Commands[index+1:]...)

  return saveSceneConfig(*config)
}

func removeScene(sceneName string) error {
  config, err := loadSceneConfig()
  if err != nil {
    return err
  }

  // Find scene index
  sceneIndex := -1
  for i, scene := range config.Scenes {
    if scene.Name == sceneName {
      sceneIndex = i
      break
    }
  }

  if sceneIndex == -1 {
    return fmt.Errorf("scene '%s' not found", sceneName)
  }

  // Remove scene
  config.Scenes = append(config.Scenes[:sceneIndex], config.Scenes[sceneIndex+1:]...)

  return saveSceneConfig(*config)
}

func executeScene(sceneName string) error {
  config, err := loadSceneConfig()
  if err != nil {
    fmt.Printf("Error loading scenes: %v\n", err)
    return err
  }

  scene := findScene(config, sceneName)
  if scene == nil {
    fmt.Printf("Scene '%s' not found\n", sceneName)
    return fmt.Errorf("scene not found")
  }

  if len(scene.Commands) == 0 {
    fmt.Printf("Scene '%s' has no commands\n", sceneName)
    return nil
  }

  fmt.Printf("Executing scene '%s' with %d commands...\n", sceneName, len(scene.Commands))

  // Execute all commands concurrently
  results := make(chan string, len(scene.Commands))

  for _, command := range scene.Commands {
    go func(cmd SceneCommand) {
      result := executeSceneCommand(cmd)
      results <- result
    }(command)
  }

  // Collect results
  successCount := 0
  for i := 0; i < len(scene.Commands); i++ {
    result := <-results
    if result == "success" {
      successCount++
    } else {
      fmt.Printf("  %s\n", result)
    }
  }

  fmt.Printf("Scene '%s' executed: %d/%d commands successful\n", sceneName, successCount, len(scene.Commands))
  return nil
}

func executeSceneCommand(command SceneCommand) string {
  // Resolve lights (supports both single lights and groups)
  lights := resolveLightIdentifiers([]string{command.Light})
  if len(lights) == 0 {
    return fmt.Sprintf("Error: Light/Group '%s' not found", command.Light)
  }

  var errors []string
  successCount := 0

  for _, light := range lights {
    var err error
    switch command.Type {
    case "on":
      err = light.On()
    case "off":
      err = light.Off()
    case "brightness":
      brightness, _ := strconv.Atoi(command.Values[0])
      err = light.Bri(uint8(brightness))
    case "color":
      r, _ := strconv.Atoi(command.Values[0])
      g, _ := strconv.Atoi(command.Values[1])
      b, _ := strconv.Atoi(command.Values[2])
      x, y := rgbToXY(uint8(r), uint8(g), uint8(b))
      err = light.Xy([]float32{x, y})
      // Handle optional brightness parameter
      if len(command.Values) == 4 {
        brightness, _ := strconv.Atoi(command.Values[3])
        if err == nil {
          err = light.Bri(uint8(brightness))
        }
      }
    }

    if err != nil {
      errors = append(errors, fmt.Sprintf("Error with '%s': %v", light.Name, err))
    } else {
      successCount++
    }
  }

  if len(errors) > 0 {
    if successCount > 0 {
      return fmt.Sprintf("Partial success for '%s' (%d/%d lights): %s", command.Light, successCount, len(lights), strings.Join(errors, "; "))
    } else {
      return fmt.Sprintf("Failed for '%s': %s", command.Light, strings.Join(errors, "; "))
    }
  }

  return "success"
}

func findScene(config *SceneConfig, sceneName string) *Scene {
  for i, scene := range config.Scenes {
    if scene.Name == sceneName {
      return &config.Scenes[i]
    }
  }
  return nil
}

func loadSceneConfig() (*SceneConfig, error) {
  var config SceneConfig
  data, err := os.ReadFile(sceneFile)
  if err != nil {
    return nil, err
  }
  err = json.Unmarshal(data, &config)
  return &config, err
}

func saveSceneConfig(config SceneConfig) error {
  data, err := json.MarshalIndent(config, "", "  ")
  if err != nil {
    return err
  }
  return os.WriteFile(sceneFile, data, 0600)
}

// Group commands
var groupCmd = &cobra.Command{
  Use:   "group",
  Short: "Manage light groups",
  Long:  `Create and manage groups of lights for easier control.`,
}

func init() {
  // Add subcommands to group
  groupCmd.AddCommand(groupAddCmd)
  groupCmd.AddCommand(groupListCmd)
  groupCmd.AddCommand(groupRemoveCmd)
  groupCmd.AddCommand(groupsListAllCmd)
}

var groupAddCmd = &cobra.Command{
  Use:   "add [group-name] [light-id/name] [light-id/name] ...",
  Short: "Add lights to a group",
  Long: `Create a new group or add lights to an existing group. Lights can be specified by ID or name.
  
Examples:
  hue group add "living-room" 1 2 3
  hue group add "bedroom" "Bedside Left" "Bedside Right"
  hue group add "kitchen" 5 "Kitchen Counter" 7`,
  Args: cobra.MinimumNArgs(2),
  Run: func(cmd *cobra.Command, args []string) {
    groupName := args[0]
    lightIdentifiers := args[1:]

    // Verify all lights exist
    allLights, err := bridge.GetLights()
    if err != nil {
      fmt.Printf("Error getting lights: %v\n", err)
      return
    }

    var validLights []string
    for _, identifier := range lightIdentifiers {
      light := resolveSingleLight(identifier, allLights)
      if light != nil {
        validLights = append(validLights, identifier)
      } else {
        fmt.Printf("Warning: Light '%s' not found, skipping\n", identifier)
      }
    }

    if len(validLights) == 0 {
      fmt.Println("No valid lights found")
      return
    }

    // Add lights to group
    if err := addLightsToGroup(groupName, validLights); err != nil {
      fmt.Printf("Error adding lights to group: %v\n", err)
      return
    }

    fmt.Printf("Added %d lights to group '%s'\n", len(validLights), groupName)
  },
}

var groupListCmd = &cobra.Command{
  Use:   "list [group-name]",
  Short: "List lights in a group",
  Long:  `Display all lights that belong to a specific group.`,
  Args:  cobra.ExactArgs(1),
  Run: func(cmd *cobra.Command, args []string) {
    groupName := args[0]

    config, err := loadGroupConfig()
    if err != nil {
      fmt.Printf("Error loading groups: %v\n", err)
      return
    }

    group := findGroup(config, groupName)
    if group == nil {
      fmt.Printf("Group '%s' not found\n", groupName)
      return
    }

    fmt.Printf("Group '%s' lights:\n", groupName)
    allLights, err := bridge.GetLights()
    if err != nil {
      fmt.Printf("Error getting lights: %v\n", err)
      return
    }

    for i, lightIdentifier := range group.Lights {
      light := resolveSingleLight(lightIdentifier, allLights)
      if light != nil {
        fmt.Printf("%d. %s (ID: %d)\n", i+1, light.Name, light.ID)
      } else {
        fmt.Printf("%d. %s (NOT FOUND)\n", i+1, lightIdentifier)
      }
    }
  },
}

var groupRemoveCmd = &cobra.Command{
  Use:   "remove [group-name] [light-index]",
  Short: "Remove a light from a group or remove entire group",
  Long: `Remove a specific light from a group by index, or remove the entire group if no index is provided.
  
Examples:
  hue group remove "living-room" 1    # Remove light at index 1
  hue group remove "living-room"      # Remove entire group`,
  Args: cobra.RangeArgs(1, 2),
  Run: func(cmd *cobra.Command, args []string) {
    groupName := args[0]

    if len(args) == 1 {
      // Remove entire group
      if err := removeGroup(groupName); err != nil {
        fmt.Printf("Error removing group: %v\n", err)
        return
      }
      fmt.Printf("Group '%s' removed\n", groupName)
    } else {
      // Remove specific light
      index, err := strconv.Atoi(args[1])
      if err != nil {
        fmt.Printf("Invalid index: %s\n", args[1])
        return
      }

      if err := removeLightFromGroup(groupName, index-1); err != nil {
        fmt.Printf("Error removing light: %v\n", err)
        return
      }
      fmt.Printf("Light %d removed from group '%s'\n", index, groupName)
    }
  },
}

var groupsListAllCmd = &cobra.Command{
  Use:   "groups",
  Short: "List all available groups",
  Long:  `Display all saved groups and their light counts.`,
  Run: func(cmd *cobra.Command, args []string) {
    config, err := loadGroupConfig()
    if err != nil {
      fmt.Printf("Error loading groups: %v\n", err)
      return
    }

    if len(config.Groups) == 0 {
      fmt.Println("No groups found")
      return
    }

    fmt.Println("Available groups:")
    for _, group := range config.Groups {
      fmt.Printf("  %s (%d lights)\n", group.Name, len(group.Lights))
    }
  },
}

// Group helper functions
func addLightsToGroup(groupName string, lightIdentifiers []string) error {
  config, err := loadGroupConfig()
  if err != nil {
    // Create new config if file doesn't exist
    config = &GroupConfig{Groups: []Group{}}
  }

  // Find or create group
  group := findGroup(config, groupName)
  if group == nil {
    config.Groups = append(config.Groups, Group{
      Name:   groupName,
      Lights: []string{},
    })
    group = &config.Groups[len(config.Groups)-1]
  }

  // Add lights to group (avoid duplicates)
  lightSet := make(map[string]bool)
  for _, light := range group.Lights {
    lightSet[light] = true
  }

  for _, lightIdentifier := range lightIdentifiers {
    if !lightSet[lightIdentifier] {
      group.Lights = append(group.Lights, lightIdentifier)
      lightSet[lightIdentifier] = true
    }
  }

  return saveGroupConfig(*config)
}

func removeLightFromGroup(groupName string, index int) error {
  config, err := loadGroupConfig()
  if err != nil {
    return err
  }

  group := findGroup(config, groupName)
  if group == nil {
    return fmt.Errorf("group '%s' not found", groupName)
  }

  if index < 0 || index >= len(group.Lights) {
    return fmt.Errorf("invalid light index %d", index+1)
  }

  // Remove light at index
  group.Lights = append(group.Lights[:index], group.Lights[index+1:]...)

  return saveGroupConfig(*config)
}

func removeGroup(groupName string) error {
  config, err := loadGroupConfig()
  if err != nil {
    return err
  }

  // Find group index
  groupIndex := -1
  for i, group := range config.Groups {
    if strings.EqualFold(group.Name, groupName) {
      groupIndex = i
      break
    }
  }

  if groupIndex == -1 {
    return fmt.Errorf("group '%s' not found", groupName)
  }

  // Remove group
  config.Groups = append(config.Groups[:groupIndex], config.Groups[groupIndex+1:]...)

  return saveGroupConfig(*config)
}

func findGroup(config *GroupConfig, groupName string) *Group {
  for i, group := range config.Groups {
    if strings.EqualFold(group.Name, groupName) {
      return &config.Groups[i]
    }
  }
  return nil
}

func loadGroupConfig() (*GroupConfig, error) {
  var config GroupConfig
  data, err := os.ReadFile(groupFile)
  if err != nil {
    return nil, err
  }
  err = json.Unmarshal(data, &config)
  return &config, err
}

func saveGroupConfig(config GroupConfig) error {
  data, err := json.MarshalIndent(config, "", "  ")
  if err != nil {
    return err
  }
  return os.WriteFile(groupFile, data, 0600)
}
