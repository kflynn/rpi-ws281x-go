package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"hash/crc32"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/rpi-ws281x/rpi-ws281x-go/pkg/glowy"
	"github.com/rpi-ws281x/rpi-ws281x-go/pkg/whisper"
)

func Announce(w *whisper.Whisper, selfID, serverID uint32, nodeNumber, procNumber, ms int, ok bool) error {
	msg := glowy.GlowMsg{
		Node:    nodeNumber,
		Process: procNumber,
		OK:      ok,
		Value:   ms,
	}

	// fmt.Printf("glowcli: sending %d/%d %d (%v)\n", nodeNumber, procNumber, ms, ok)

	jsonData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	// fmt.Printf("glowcli: JSON payload: %s\n", string(jsonData))

	return w.Send(serverID, glowy.CmdActivity, jsonData)
}

func run_simulation(w *whisper.Whisper, nodeNumber int, selfID uint32, serverID uint32) {
	fmt.Printf("glowcli: running in simulation mode\n")

	buckets := []int{10, 25, 50, 100, 250, 500}

	for {
		procNumber := rand.Intn(4)             // Random process 0-3
		ms := buckets[rand.Intn(len(buckets))] // Random element from buckets
		ok := rand.Float32() < 0.8             // 80% chance of success

		err := Announce(w, selfID, serverID, nodeNumber, procNumber, ms, ok)

		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to send announcement: %v\n", err)
		}

		// Wait before next iteration
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}
}

func main() {
	var whisperAddr = flag.String("whisper", "", "IP address for whisper server")
	var simMode = flag.Bool("sim", false, "Run in simulation mode")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [--whisper <ip-addr>] --sim <node>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "       %s [--whisper <ip-addr>] <node> <proc> <ms> <ok>\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()

	args := flag.Args()

	var nodeNumber, procNumber, ms int
	var ok bool
	var err error

	if *simMode {
		if len(args) != 1 {
			fmt.Fprintf(os.Stderr, "Error: --sim mode requires exactly one node argument\n")
			flag.Usage()
			os.Exit(1)
		}

		nodeNumber, err = strconv.Atoi(args[0])

		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid node number: %v\n", err)
			os.Exit(1)
		}
	} else {
		if len(args) != 4 {
			fmt.Fprintf(os.Stderr, "Error: requires node, proc, ms, and ok arguments\n")
			flag.Usage()
			os.Exit(1)
		}

		nodeNumber, err = strconv.Atoi(args[0])

		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid node number: %v\n", err)
			os.Exit(1)
		}

		procNumber, err = strconv.Atoi(args[1])

		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid proc number: %v\n", err)
			os.Exit(1)
		}

		ms, err = strconv.Atoi(args[2])

		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid ms value: %v\n", err)
			os.Exit(1)
		}

		ok = args[3] == "true"
	}

	if len(os.Args) == 2 {
		if os.Args[1] == "--help" || os.Args[1] == "-h" {
			fmt.Printf("Usage: %s --sim <node>\n", os.Args[0])
			fmt.Printf("       %s <node> <proc> <ms> <ok>\n", os.Args[0])
			os.Exit(0)
		}
	}

	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Invalid arguments. Use --help for usage.\n")
		os.Exit(1)
	}

	// Server we'll send to
	serverID := crc32.ChecksumIEEE([]byte("glowsrv"))

	// Local ID based on node number
	idStr := fmt.Sprintf("glowcli-%d", nodeNumber)
	selfID := crc32.ChecksumIEEE([]byte(idStr))

	fmt.Printf("glowcli: sending to server ID 0x%08X\n", serverID)

	w, err := whisper.NewWhisperWithOptions(*whisperAddr, whisper.DefaultPort)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create Whisper: %v\n", err)
		os.Exit(1)
	}

	defer w.Close()

	w.SetID(selfID)

	fmt.Printf("Whisper enabled: %s\n", w.String())

	if *simMode {
		run_simulation(w, nodeNumber, selfID, serverID)
	}

	err = Announce(w, selfID, serverID, nodeNumber, procNumber, ms, ok)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to send announcement: %v\n", err)
		os.Exit(1)
	}
}
