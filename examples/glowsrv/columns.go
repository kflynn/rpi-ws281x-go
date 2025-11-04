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

func (c *Column) Set(color uint32, height int) {
	if c.State == ColumnStateCycling {
		return
	}

	c.ActiveTime = time.Now()
	c.State = ColumnStateActive
	c.Color = color
	c.Height = height
}

func (c *Column) IsActive() bool {
	return (c.State == ColumnStateActive)
}

func (c *Column) IsCycling() bool {
	return (c.State == ColumnStateCycling)
}

func (c *Column) Decay(now time.Time) {
	if c.Height > 0 {
		c.Height -= 4

		if c.Height < 0 {
			c.Height = 0
		}
	}

	if c.Height == 0 && now.Sub(c.ActiveTime) > 2*time.Second {
		c.State = ColumnStateIdle
	}
}

func (c *Column) Cycle() {
	c.State = ColumnStateCycling

	go func() {
		fmt.Printf("Executing: /home/flynn/bin/cycle %d %d\n", c.Node, c.Process)

		cmd := exec.Command("/home/flynn/bin/cycle", fmt.Sprintf("%d", c.Node), fmt.Sprintf("%d", c.Process))

		output, err := cmd.CombinedOutput()

		if err != nil {
			fmt.Fprintf(os.Stderr, "cycle failed: %v\n%s\n", err, output)
			// } else {
			// 	fmt.Printf("cycle output: %s\n", output)
		}
	}()
}
