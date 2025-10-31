package main

import (
	"encoding/json"
	"fmt"
	"hash/crc32"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"strings"
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
	GSrvStateWin    = 3

	GSrvUpdatesPerSecond = 10
	GSrvSecondsForTopLED = 2
)

// These are Faces colors, from https://sronpersonalpages.nl/~pault
const (
	ColorBlue    = 0x66CCEE
	ColorBlueHue = 195.0 / 360.0
	ColorBlueSat = 0.57

	ColorRed    = 0xEE6677
	ColorRedHue = 353.0 / 360.0
	ColorRedSat = 0.57

	ColorYellow = 0xFFFF00
	ColorGreen  = 0x00FF00
	ColorWhite  = 0xFFFFFF
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

	topLEDCol      int
	topLEDColor    uint32
	topLEDUpdateCh chan struct{}
}

func NewGlowSrv(rows, cols int, leds *LEDs) (*GlowSrv, error) {
	gs := &GlowSrv{
		state:          GSrvStateSnake,
		delay:          0,
		lastActivity:   time.Now(),
		rows:           rows,
		cols:           cols,
		leds:           leds,
		columns:        make([]Column, cols),
		snake:          Snake{},
		topLEDCol:      -1,
		topLEDColor:    0,
		topLEDUpdateCh: make(chan struct{}, 1),
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

		case GSrvStateWin:
			// From Win state, clear everything
			gs.leds.Fill(0)

		default:
			// Normal state. Clear on any state change.
			gs.leds.Fill(0)
		}

		// From anything to the Snake state, reset the snake.
		if state == GSrvStateSnake {
			gs.initSnake()
		}

		// Entering Win state: clear all and paint WIN, set 10-second delay
		if state == GSrvStateWin {
			gs.leds.Fill(0)
			gs.paintWin()
			gs.delay = 10 * GSrvUpdatesPerSecond
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
		gs.paintTopLED()

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

func (gs *GlowSrv) clearTopLED() {
	if gs.topLEDCol >= 0 {
		gs.leds.SetPixel(gs.topLEDCol, 0, 0)
	}
}

func (gs *GlowSrv) paintTopLED() {
	if gs.topLEDCol >= 0 {
		gs.leds.SetPixel(gs.topLEDCol, 0, gs.topLEDColor)
	}
}

const WIN = `
X       X XXXXX X      X  X
X       X   X   XX     X  X
X       X   X   X X    X  X
X       X   X   X  X   X  X
 X     X    X   X   X  X  X
 X  X  X    X   X    X X
  X X X     X   X     XX  X
   X X    XXXXX X      X  X
`

func (gs *GlowSrv) paintWin() {
	// Paint "WIN" in blue letters centered on the display

	color := uint32(ColorBlue)

	lines := strings.Split(strings.TrimSpace(WIN), "\n")

	startCol := 2

	for row, line := range lines {
		// fmt.Printf("WIN line %d: %s\n", row, line)
		for colOffset, char := range line {
			if char == 'X' {
				col := startCol + colOffset
				if col < gs.cols && row < gs.rows {
					// fmt.Printf("%d ", col)
					gs.leds.SetPixel(col, row, color)
				}
			}
		}
		// fmt.Println()
	}
}

func (gs *GlowSrv) LEDHit() {
	// 2. Get the node and process from the current column (before clearing)
	if gs.topLEDCol >= 0 && gs.topLEDCol < len(gs.columns) {
		col := &gs.columns[gs.topLEDCol]
		node := col.Node
		process := col.Process

		// 1. Turn off the top LED
		gs.clearTopLED()

		// Immediately pick a new top LED and reset the timer
		gs.UpdateTopLED()
		select {
		case gs.topLEDUpdateCh <- struct{}{}:
		default:
		}

		// 3. Execute the kubectl command in a goroutine
		go func(node, process int) {
			fmt.Printf("Executing: /home/flynn/bin/cycle %d %d\n", node, process)

			cmd := exec.Command("/home/flynn/bin/cycle", fmt.Sprintf("%d", node), fmt.Sprintf("%d", process))

			output, err := cmd.CombinedOutput()

			if err != nil {
				fmt.Fprintf(os.Stderr, "cycle failed: %v\n%s\n", err, output)
				// } else {
				// 	fmt.Printf("cycle output: %s\n", output)
			}
		}(node, process)
	}
}

func (gs *GlowSrv) UpdateTopLED() {
	// Clear previous top LED if set
	gs.clearTopLED()

	// Find all active columns
	activeColumns := []int{}
	for col, column := range gs.columns {
		if column.Active && column.Height > 0 {
			activeColumns = append(activeColumns, col)
		}
	}

	// If no active columns, go to Win state
	if len(activeColumns) == 0 {
		gs.topLEDCol = -1
		gs.topLEDColor = 0
		gs.UseState(GSrvStateWin)
		return
	}

	// Pick a random active column and color. Both must be different from
	// their current values.
	col := gs.topLEDCol

	for col == gs.topLEDCol {
		col = activeColumns[rand.Intn(len(activeColumns))]
	}

	colors := []uint32{ColorBlue, ColorYellow, ColorRed, ColorGreen, ColorWhite}

	color := gs.topLEDColor

	for color == gs.topLEDColor {
		color = colors[rand.Intn(len(colors))]
	}

	// fmt.Printf("Top LED: col=%d color=0x%06X\n", col, color)

	gs.topLEDCol = col
	gs.topLEDColor = color
	gs.paintTopLED()
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

	case GSrvStateWin:
		gs.delay--

		if gs.delay <= 0 {
			gs.UseState(GSrvStateIdle)
		}

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

	// Don't process activity during Win state
	if gs.state != GSrvStateWin {
		gs.UseState(GSrvStateNormal)
		gs.SetColumn(msg.Node, msg.Process, color, capped_height)
	}
}

func (gs *GlowSrv) handleIdleCommand(data []byte) {
	// Switch to Snake state.
	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	gs.UseState(GSrvStateSnake)
}

func (gs *GlowSrv) handleButtonPress(key string) {
	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	// Ignore button presses during Win state
	if gs.state == GSrvStateWin {
		return
	}

	// fmt.Printf("handleButtonPress: key=%s topLEDCol=%d topLEDColor=0x%06X\n", key, gs.topLEDCol, gs.topLEDColor)

	// SPACE always generates a hit (for testing)
	if key == "SPACE" {
		if gs.topLEDCol >= 0 {
			fmt.Printf("✓ SPACE hit (testing)!\n")
			gs.LEDHit()
		}
		return
	}

	// Check if key matches the top LED color
	var keyMatches bool

	switch gs.topLEDColor {
	case ColorBlue:
		keyMatches = (key == "B")
	case ColorYellow:
		keyMatches = (key == "Y")
	case ColorRed:
		keyMatches = (key == "R")
	case ColorGreen:
		keyMatches = (key == "G")
	case ColorWhite:
		keyMatches = (key == "W")
	}

	if keyMatches && gs.topLEDCol >= 0 {
		fmt.Printf("✓ hit!\n")
		gs.LEDHit()
	}
}

func main() {
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

	gsrv, err := NewGlowSrv(GSrvRows, GSrvCols, leds)

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
				err := gsrv.Update()

				if err != nil {
					fmt.Fprintf(os.Stderr, "Failed to update GlowSrv: %v\n", err)
				}

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
				gsrv.mutex.Lock()
				if gsrv.state == GSrvStateNormal {
					gsrv.UpdateTopLED()
				}
				gsrv.mutex.Unlock()

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
		go readKeyboard(device, gsrv, done)
	}

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
			close(done)
			return
		}
	}
}
