package main

import (
	"fmt"
	"os"
	"os/exec"
	"time"
)

const (
	ColumnStateIdle    = 0
	ColumnStateActive  = 1
	ColumnStateCycling = 2
)

type Column struct {
	ActiveTime time.Time
	State      int
	Color      uint32
	Height     int
	Node       int
	Process    int
}

func (c *Column) Clear() {
	c.ActiveTime = time.Time{}
	c.State = ColumnStateIdle
	c.Color = 0
	c.Height = 0
}

func (c *Column) Name() string {
	colState := "I"

	if c.IsActive() {
		colState = "A"
	} else if c.IsCycling() {
		colState = "C"
	}

	return fmt.Sprintf("%d,%d [%s]", c.Node, c.Process, colState)
}

func (c *Column) Set(color uint32, height int) {
	if c.State == ColumnStateCycling {
		return
	}

	c.ActiveTime = time.Now()
	c.State = ColumnStateActive
	c.Color = color
	c.Height = height

	// if c.IsInteresting() {
	// 	fmt.Printf("S %s -> %06x height %d\n", c.Name(), color, height)
	// }
}

func (c *Column) SetActive() {
	// fmt.Printf("Reactivating column %d %d\n", c.Node, c.Process)
	c.State = ColumnStateActive
}

func (c *Column) IsActive() bool {
	return (c.State == ColumnStateActive)
}

func (c *Column) IsCycling() bool {
	return (c.State == ColumnStateCycling)
}

func (c *Column) IsInteresting() bool {
	return (c.Node == 1) && (c.Process == 0)
}

func (c *Column) Decay(now time.Time) {
	if c.Height > 0 {
		if c.IsCycling() {
			c.Height -= 10
		} else {
			c.Height -= 2
		}

		if c.Height < 0 {
			c.Height = 0
		}
	}

	if c.Height == 0 && now.Sub(c.ActiveTime) > 2*time.Second {
		c.State = ColumnStateIdle
	}

	// if (c.Height > 0) && c.IsInteresting() {
	// 	fmt.Printf("D %s -> %06x height %d\n", c.Name(), c.Color, c.Height)
	// }
}

func (c *Column) Cycle(gs *GlowSrv) {
	// Don't spawn multiple cycles for the same column
	if c.State == ColumnStateCycling {
		return
	}

	c.State = ColumnStateCycling

	go func() {
		fmt.Printf("Executing: /home/flynn/bin/cycle %d %d\n", c.Node, c.Process)

		// Open log file for appending
		logFile, err := os.OpenFile("/home/flynn/cycle.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open cycle.log: %v\n", err)
			return
		}
		defer logFile.Close()

		// Run cycle command in background with output redirected
		cmd := exec.Command("/home/flynn/bin/cycle", fmt.Sprintf("%d", c.Node), fmt.Sprintf("%d", c.Process))
		cmd.Stdout = logFile
		cmd.Stderr = logFile

		err = cmd.Start()

		if err != nil {
			fmt.Fprintf(os.Stderr, "cycle launch failed: %v\n", err)
		}

		// Wait 2 seconds then set state back to Active
		time.Sleep(2 * time.Second)
		gs.eventQueue.Send(Event{Cmd: EventCmdReactivate, Column: c})
	}()
}
