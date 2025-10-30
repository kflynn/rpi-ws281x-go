// Copyright 2018 Jacques Supcik / HEIA-FR
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"os"
	"time"

	ws2811 "github.com/rpi-ws281x/rpi-ws281x-go/pkg/ws2811"
)

const (
	brightness = 128
	ledCounts  = 256
	sleepTime  = 20
)

type wsEngine interface {
	Init() error
	Render() error
	Wait() error
	Fini()
	Leds(channel int) []uint32
}

func hsv_to_rgb(h, s, v float64) (r, g, b float64) {
	if s == 0 {
		return v, v, v
	}

	h = h * 6.0
	i := int(h)
	f := h - float64(i)
	p := v * (1.0 - s)
	q := v * (1.0 - s*f)
	t := v * (1.0 - s*(1.0-f))

	switch i {
	case 0:
		return v, t, p
	case 1:
		return q, v, p
	case 2:
		return p, v, t
	case 3:
		return p, q, v
	case 4:
		return t, p, v
	default:
		return v, p, q
	}
}

func floatToRGB(r, g, b float64) uint32 {
	red := uint32(r * 255)
	green := uint32(g * 255)
	blue := uint32(b * 255)

	rgb := (red << 16) | (green << 8) | blue
	return rgb
}

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}

type rainbow struct {
	ws wsEngine
}

func (rb *rainbow) setup() error {
	return rb.ws.Init()
}

func (rb *rainbow) display(color uint32) error {
	for i := 0; i < len(rb.ws.Leds(0)); i++ {
		rb.ws.Leds(0)[i] = color
		time.Sleep(sleepTime * time.Millisecond)
	}
	return nil
}

func main() {
	opt := ws2811.DefaultOptions
	opt.Channels[0].Brightness = brightness
	opt.Channels[0].LedCount = ledCounts

	dev, err := ws2811.MakeWS2811(&opt)
	checkError(err)

	rb := &rainbow{
		ws: dev,
	}

	checkError(rb.setup())

	defer func() {
		for i := 0; i < len(rb.ws.Leds(0)); i++ {
			rb.ws.Leds(0)[i] = uint32(0x000000)
		}

		if err := rb.ws.Render(); err != nil {
			fmt.Errorf("error rendering in defer: %s", err)
			os.Exit(1)
		}

		dev.Fini()
	}()

	for count := 0; count < 3; count++ {
		for angle := 0.0; angle < 360.0; angle += 1.0 {
			// fmt.Printf("%d - %.1f\n", count, angle)

			r, g, b := hsv_to_rgb(angle/360.0, 1.0, 1.0)

			for i := 0; i < len(rb.ws.Leds(0)); i++ {
				rb.ws.Leds(0)[i] = floatToRGB(r, g, b)
			}

			if err := rb.ws.Render(); err != nil {
				fmt.Errorf("error rendering: %s", err)
				os.Exit(1)
			}

			time.Sleep(sleepTime * time.Millisecond)
		}
	}
}
