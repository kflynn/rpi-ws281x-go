package main

import (
	"fmt"
	"hash/crc32"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rpi-ws281x/rpi-ws281x-go/pkg/glowy"
	"github.com/rpi-ws281x/rpi-ws281x-go/pkg/whisper"
)

func main() {
	// Our event queue
	eventQueue := NewEventQueue(100)

	// Handle SIGINT/SIGTERM for graceful shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan struct{})

	GSrvRows := 8
	GSrvCols := 32

	mapping := make([]int, GSrvCols*GSrvRows)

	for col := 0; col < GSrvCols; col += 2 {
		for row := 0; row < GSrvRows; row++ {
			mapping[(col*GSrvRows)+row] = (col*GSrvRows + row)
			mapping[((col+1)*GSrvRows)+row] = ((col+1)*GSrvRows + (GSrvRows - 1 - row))
		}
	}

	whisperAddr := whisper.DefaultGroupAddr

	if len(os.Args) > 1 {
		whisperAddr = os.Args[1]
	}

	w, err := whisper.NewWhisperWithOptions(whisperAddr, whisper.DefaultPort)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create Whisper: %v\n", err)
		os.Exit(1)
	}

	defer w.Close()

	// Calculate our ID from "glowsrv"
	serverID := crc32.ChecksumIEEE([]byte("glowsrv"))
	w.SetID(serverID)

	fmt.Printf("Whisper enabled: %s\n", w.String())

	err = w.Listen()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start Whisper listener: %v\n", err)
		os.Exit(1)
	}

	leds, err := NewLEDs(GSrvRows, GSrvCols, &mapping, 128)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create LEDs: %v\n", err)
		os.Exit(1)
	}
	defer leds.Close()

	err = leds.Clear()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to clear LEDs: %v\n", err)
		os.Exit(1)
	}

	gsrv, err := NewGlowSrv(eventQueue, GSrvRows, GSrvCols, leds)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create GlowSrv: %v\n", err)
		os.Exit(1)
	}

	// Start update timer
	go func() {
		ticker := time.NewTicker((1000 * time.Millisecond) / GSrvUpdatesPerSecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				eventQueue.Send(Event{Cmd: EventCmdUpdate})

			case <-done:
				return
			}
		}
	}()

	fmt.Printf("update task started\n")

	// Start top LED update timer
	go func() {
		ticker := time.NewTicker(GSrvSecondsForTopLED * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				eventQueue.Send(Event{Cmd: EventCmdNewTopLED})

			case <-gsrv.topLEDUpdateCh:
				// Reset the timer when a hit occurs
				ticker.Reset(GSrvSecondsForTopLED * time.Second)

			case <-done:
				return
			}
		}
	}()

	// Start keyboard reader
	// Try to auto-detect keyboard, or use command line arg, or default to event0
	kbdDevices := findKeyboardDevices()

	if len(os.Args) > 2 {
		kbdDevices = append(kbdDevices, os.Args[2])
	}

	if len(kbdDevices) == 0 {
		kbdDevices = append(kbdDevices, "/dev/input/event0")
	}

	for _, device := range kbdDevices {
		go readKeyboard(device, eventQueue, done)
	}

	// Start event queue processor
	go eventQueue.Run(gsrv)

	// Main loop: process Whisper messages
	fmt.Println("glowsrv: waiting for susurri...")
	for {
		select {
		case srs := <-w.RecvChan:
			if srs.Cmd == glowy.CmdActivity {
				eventQueue.Send(Event{Cmd: EventCmdActivity, Data: srs.Data})
			} else if srs.Cmd == glowy.CmdIdle {
				eventQueue.Send(Event{Cmd: EventCmdNewState, NewState: GSrvStateSnake})
			} else {
				fmt.Printf("Unknown command 0x%04X\n", srs.Cmd)
			}

		case sig := <-sigs:
			fmt.Printf("Received signal %v, shutting down.\n", sig)
			close(done)
			return
		}
	}
}
