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

	ws2811 "github.com/rpi-ws281x/rpi-ws281x-go/pkg/ws2811"
)

type wsEngine interface {
	Init() error
	Render() error
	Wait() error
	Fini()
	Leds(channel int) []uint32
}

type LEDs struct {
	ws wsEngine

	rows int
	cols int

	mapping *[]int
}

func NewLEDs(rows, cols int, mapping *[]int, brightness int) (*LEDs, error) {
	if mapping != nil {
		if rows*cols != len(*mapping) {
			return nil, fmt.Errorf("mapping length %d does not match rows*cols %d", len(*mapping), rows*cols)
		}
	}

	numLEDs := rows * cols

	opt := ws2811.DefaultOptions
	opt.Channels[0].Brightness = brightness
	opt.Channels[0].LedCount = numLEDs

	dev, err := ws2811.MakeWS2811(&opt)

	if err != nil {
		return nil, err
	}

	err = dev.Init()

	if err != nil {
		return nil, err
	}

	leds := &LEDs{
		ws:      dev,
		mapping: mapping,
		rows:    rows,
		cols:    cols,
	}

	return leds, nil
}

func (leds LEDs) Clear() error {
	leds.Fill(0)

	// for i := 0; i < 16; i++ {
	// 	leds.ws.Leds(0)[i] = 0x00FF00
	// }

	return leds.Render()
}

func (leds LEDs) Close() {
	_ = leds.Clear()
	leds.ws.Fini()
}

func (leds LEDs) Render() error {
	return leds.ws.Render()
}

func (leds LEDs) Fill(color uint32) {
	for i := 0; i < len(leds.ws.Leds(0)); i++ {
		leds.ws.Leds(0)[i] = color
	}
}

func (leds LEDs) SetPixel(x, y int, color uint32) {
	index := x*leds.rows + y
	leds.SetIndex(index, color)
}

func (leds LEDs) SetIndex(index int, color uint32) {
	if leds.mapping != nil {
		index = (*leds.mapping)[index]
	}

	leds.ws.Leds(0)[index] = color
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
