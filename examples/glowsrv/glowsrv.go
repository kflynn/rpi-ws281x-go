package main

import (
	"encoding/json"
	"fmt"
	"hash/crc32"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/rpi-ws281x/rpi-ws281x-go/pkg/glowy"
	"github.com/rpi-ws281x/rpi-ws281x-go/pkg/whisper"
)

const (
	GSrvStateIdle   = 0
	GSrvStateSnake  = 1
	GSrvStateNormal = 2
)

const (
	ColorBlue    = 0x66CCEE
	ColorBlueHue = 195.0 / 360.0
	ColorBlueSat = 0.57

	ColorRed    = 0xEE6677
	ColorRedHue = 353.0 / 360.0
	ColorRedSat = 0.57
)

type Column struct {
	Active     bool
	ActiveTime time.Time
	Color      uint32
	Height     int
	Node       int
	Process    int
}

func (c *Column) Clear() {
	c.Active = false
	c.Color = 0
	c.Height = 0
}

func (c *Column) Set(color uint32, height int) {
	c.Active = true
	c.ActiveTime = time.Now()
	c.Color = color
	c.Height = height
}

func (c *Column) Decay() {
	if c.Height > 0 {
		c.Height--
	}
}

type GlowSrv struct {
	state int
	delay int

	lastActivity time.Time

	rows int
	cols int

	leds *LEDs

	mutex   sync.RWMutex
	columns []Column

	snake Snake
}

func NewGlowSrv(rows, cols int, leds *LEDs) (*GlowSrv, error) {
	gs := &GlowSrv{
		state:        GSrvStateSnake,
		delay:        0,
		lastActivity: time.Now(),
		rows:         rows,
		cols:         cols,
		leds:         leds,
		columns:      make([]Column, cols),
		snake:        Snake{},
	}

	gs.initSnake()

	return gs, nil
}

func (gs *GlowSrv) ClearAll() {
	gs.leds.Fill(0)
	for col := 0; col < gs.cols; col++ {
		gs.columns[col].Clear()
	}
}

func (gs *GlowSrv) UseState(state int) {
	// gs.mutex.Lock()
	// defer gs.mutex.Unlock()

	if gs.state != state {
		// fmt.Printf("GlowSrv: changing state from %d to %d\n", gs.state, state)

		switch gs.state {
		case GSrvStateIdle:
			// From Idle to anything, we clear all LEDs.
			gs.leds.Fill(0)

		case GSrvStateSnake:
			// From Snake, going to Normal, clear the board entirely.
			if state == GSrvStateNormal {
				gs.ClearAll()
			}

		default:
			// Normal state. Clear on any state change.
			gs.leds.Fill(0)
		}

		// From anything to the Snake state, reset the snake.
		if state == GSrvStateSnake {
			gs.initSnake()
		}
	}

	gs.state = state
}

func (gs *GlowSrv) initSnake() {
	gs.snake.Init(gs.rows, gs.cols)
}

func (gs *GlowSrv) UpdateSnake() {
	collision := gs.snake.Update()

	for _, cell := range gs.snake.Body {
		if (cell.X >= 0) && (cell.Y >= 0) {
			value := float64(cell.Age) / float64(SnakeLength)
			hue := ColorBlueHue
			sat := ColorBlueSat

			if collision {
				hue = ColorRedHue
				sat = ColorRedSat

				value = 0.5 + (value * 0.5)
			}

			if cell.Age > 0 {
				color := floatToRGB(hsv_to_rgb(
					float64(hue),
					float64(sat),
					value,
				))

				gs.leds.SetPixel(cell.X, cell.Y, color)
			} else {
				gs.leds.SetPixel(cell.X, cell.Y, 0)
			}
		}
	}

	if collision {
		// fmt.Printf("Snake collision! Going to idle\n")
		gs.delay = 20 // Number of update cycles to stay in idle
		gs.UseState(GSrvStateIdle)
	}
}

func (gs *GlowSrv) SetColumn(node int, process int, color uint32, height int) {
	// gs.mutex.Lock()
	// defer gs.mutex.Unlock()

	// Calculate column from node and process
	col := (((node - 1) * 5) + process) + 1

	gs.columns[col].Set(color, height)
	gs.columns[col].Node = node
	gs.columns[col].Process = process
}

func (gs *GlowSrv) Render() error {
	// In Normal state, we'll need to re-render the whole display.
	if gs.state == GSrvStateNormal {
		gs.leds.Fill(0)

		for col, column := range gs.columns {
			height := column.Height

			for row := 0; row < height && row < gs.rows; row++ {
				gs.leds.SetPixel(col, gs.rows-1-row, column.Color)
			}
		}
	}

	return gs.leds.Render()
}

func (gs *GlowSrv) Decay() {
	for col := range gs.columns {
		gs.columns[col].Decay()
	}
}

func (gs *GlowSrv) Update() error {
	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	err := gs.Render()

	switch gs.state {
	case GSrvStateIdle:
		gs.delay--

		if gs.delay <= 0 {
			gs.UseState(GSrvStateSnake)
		}

	case GSrvStateSnake:
		gs.UpdateSnake()

	default:
		gs.Decay()

		if time.Since(gs.lastActivity) > 30*time.Second {
			gs.UseState(GSrvStateSnake)
		}
	}

	return err
}

func (gs *GlowSrv) handleActivityCommand(data []byte) {
	var msg glowy.GlowMsg

	err := json.Unmarshal(data, &msg)

	if err != nil {
		fmt.Printf("Failed to unmarshal JSON: %v\n", err)
		return
	}

	// Update last activity time
	gs.lastActivity = time.Now()

	color := uint32(0x66CCEE)

	if !msg.OK {
		color = 0xEE6677
	}

	if msg.Node < 1 || msg.Node > 6 || msg.Process < 0 || msg.Process > 4 {
		fmt.Printf("Invalid node/process number: %d/%d\n", msg.Node, msg.Process)
		return
	}

	height := msg.Value / 10
	capped_height := max(min(height, gs.rows-2), 1)

	// fmt.Printf("n%dp%d %v/%d -> %d\n", msg.Node, msg.Process, msg.OK, msg.Value, capped_height)

	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	gs.UseState(GSrvStateNormal)
	gs.SetColumn(msg.Node, msg.Process, color, capped_height)
}

func (gs *GlowSrv) handleIdleCommand(data []byte) {
	// Switch to Snake state.
	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	gs.UseState(GSrvStateSnake)
}

func main() {
	// Handle SIGINT/SIGTERM for graceful shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

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

	gsrv, err := NewGlowSrv(GSrvRows, GSrvCols, leds)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create GlowSrv: %v\n", err)
		os.Exit(1)
	}

	// Start update timer
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				err := gsrv.Update()

				if err != nil {
					fmt.Fprintf(os.Stderr, "Failed to update GlowSrv: %v\n", err)
				}

			case <-sigs:
				return
			}
		}
	}()

	fmt.Println("glowsrv: waiting for susurri...")
	for {
		select {
		case srs := <-w.RecvChan:
			// fmt.Printf("Received susurrus: dest=0x%08X source=0x%08X, Cmd=0x%04X, Nonce=%d, Length=%d, Data=%X\n", srs.Dest, srs.Source, srs.Cmd, srs.Nonce, srs.Length, srs.Data)

			if srs.Cmd == glowy.CmdActivity {
				gsrv.handleActivityCommand(srs.Data)
			} else if srs.Cmd == glowy.CmdIdle {
				// Handle idle command
				gsrv.handleIdleCommand(srs.Data)
			} else {
				fmt.Printf("Unknown command 0x%04X\n", srs.Cmd)
			}

		case sig := <-sigs:
			fmt.Printf("Received signal %v, shutting down.\n", sig)
			return
		}
	}
}
