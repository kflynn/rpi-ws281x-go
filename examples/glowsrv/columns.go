package main

import (
	"fmt"
	"os"
	"os/exec"
	"time"
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
		c.Height -= 4

		if c.Height < 0 {
			c.Height = 0
		}
	}
	}
}

func (c *Column) Cycle() {
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
