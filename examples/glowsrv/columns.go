package main

import (
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
		c.Height--
	}
}
