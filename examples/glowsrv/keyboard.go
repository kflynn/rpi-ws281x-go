package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"
)

// Linux input event structure
type inputEvent struct {
	Time  syscall.Timeval // time in seconds since epoch
	Type  uint16          // event type
	Code  uint16          // event code
	Value int32           // event value
}

// Key codes for B, G, R, Y, W, SPACE
const (
	KEY_B     = 48 // B key
	KEY_G     = 34 // G key
	KEY_R     = 19 // R key
	KEY_Y     = 21 // Y key
	KEY_W     = 17 // W key
	KEY_SPACE = 57 // Space key

	EV_KEY     = 1 // Key event type
	KEY_PRESS  = 1 // Key pressed
	KEY_REPEAT = 2 // Key held down (auto-repeat)
)

func findKeyboardDevice() string {
	// Parse /proc/bus/input/devices to find keyboard
	f, err := os.Open("/proc/bus/input/devices")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open /proc/bus/input/devices: %v\n", err)
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var currentName string
	var currentHandlers string
	var fallbackDevice string

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "N: Name=") {
			// Extract device name
			currentName = strings.TrimPrefix(line, "N: Name=")
			currentName = strings.Trim(currentName, "\"")

			// Skip HDMI devices
			if strings.Contains(currentName, "hdmi") {
				fmt.Printf("skipping HDMI device: %s\n", currentName)
				currentName = ""
				currentHandlers = ""
				continue
			}

			fmt.Printf("device block: %s\n", currentName)
			currentHandlers = ""
		} else if currentName != "" && strings.HasPrefix(line, "H: Handlers=") {
			// Extract handlers
			currentHandlers = strings.TrimPrefix(line, "H: Handlers=")
		} else if line == "" && currentHandlers != "" {
			// End of device block - check if this was a keyboard
			fmt.Printf("check %s: handlers=%s\n", currentName, currentHandlers)

			if strings.Contains(currentHandlers, "kbd") {
				// Extract event number from handlers
				fields := strings.Fields(currentHandlers)
				var eventDevice string
				for _, field := range fields {
					if strings.HasPrefix(field, "event") {
						eventDevice = fmt.Sprintf("/dev/input/%s", field)
						break
					}
				}

				if eventDevice != "" {
					// Prefer keyboards without joystick handlers
					if !strings.Contains(currentHandlers, "js") {
						fmt.Printf("Found keyboard: %s (%s)\n", currentName, eventDevice)
						return eventDevice
					} else if fallbackDevice == "" {
						// Keep as fallback in case we don't find a pure keyboard
						fallbackDevice = eventDevice
					}
				}
			}
			// Reset for next device
			currentName = ""
			currentHandlers = ""
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading /proc/bus/input/devices: %v\n", err)
	}

	// Return fallback if we only found kbd+js devices
	if fallbackDevice != "" {
		fmt.Printf("Found keyboard (with joystick): %s\n", fallbackDevice)
	}
	return fallbackDevice
}

func readKeyboard(device string, gsrv *GlowSrv, done chan struct{}) {
	f, err := os.Open(device)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open keyboard device %s: %v\n", device, err)
		fmt.Fprintf(os.Stderr, "Keyboard input disabled. Run with sudo or add user to 'input' group.\n")
		return
	}
	defer f.Close()

	fmt.Printf("Keyboard input enabled on %s\n", device)

	var event inputEvent

	for {
		select {
		case <-done:
			return
		default:
			// Read one input event
			err := binary.Read(f, binary.LittleEndian, &event)
			if err != nil {
				// Check if we should exit
				select {
				case <-done:
					return
				default:
					fmt.Fprintf(os.Stderr, "Error reading keyboard event: %v\n", err)
					time.Sleep(100 * time.Millisecond)
					continue
				}
			}

			// Only handle key press events (not release or repeat)
			if event.Type == EV_KEY && event.Value == KEY_PRESS {
				// fmt.Printf("Key pressed: code=%d\n", event.Code)
				var key string
				switch event.Code {
				case KEY_B:
					key = "B"
				case KEY_G:
					key = "G"
				case KEY_R:
					key = "R"
				case KEY_Y:
					key = "Y"
				case KEY_W:
					key = "W"
				case KEY_SPACE:
					key = "SPACE"
				default:
					continue
				}
				gsrv.handleButtonPress(key)
			}
		}
	}
}
