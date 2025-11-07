package main

import (
	"fmt"
	"os"
	"runtime"
)

type Event struct {
	Cmd      int
	Data     []byte
	Key      string
	NewState int
	NewDelay int
	Column   *Column
}

const (
	EventCmdActivity   = 1 // Process reporting activity
	EventCmdRender     = 2 // Time to render!
	EventCmdNewTopLED  = 3 // Time to pick a new top LED!
	EventCmdKeyPress   = 4 // Keypress arrived
	EventCmdUpdate     = 5 // Time to update the world!
	EventCmdNewState   = 6 // Change state
	EventCmdReactivate = 7 // Reactivate a column
)

type EventQueue struct {
	events chan Event
}

func NewEventQueue(size int) *EventQueue {
	return &EventQueue{
		events: make(chan Event, size),
	}
}

func (eq *EventQueue) Send(event Event) {
	// fmt.Printf("EventQueue: sending event cmd=%d\n", event.Cmd)
	select {
	case eq.events <- event:
	default:
		// Dump goroutine stacks to /tmp/dump for debugging
		buf := make([]byte, 1<<20)
		n := runtime.Stack(buf, true)
		_ = os.WriteFile("/tmp/dump", buf[:n], 0644)

		fmt.Printf("EventQueue: send failed, channel full\n")
		os.Exit(1)
	}
}

func (eq *EventQueue) Run(gs *GlowSrv) {
	for event := range eq.events {
		switch event.Cmd {
		case EventCmdActivity:
			gs.handleActivityCommand(event.Data)
		case EventCmdKeyPress:
			gs.handleButtonPress(event.Key)
		case EventCmdNewTopLED:
			gs.UpdateTopLED(false)
		case EventCmdRender:
			gs.Render()
		case EventCmdUpdate:
			gs.Update()
		case EventCmdNewState:
			gs.SwitchState(event.NewState, event.NewDelay)
		case EventCmdReactivate:
			if event.Column != nil {
				event.Column.SetActive()
			}
		}
	}
}
