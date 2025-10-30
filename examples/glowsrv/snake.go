package main

import "math/rand"

const (
	SnakeLength = 20
)

type SnakeCell struct {
	Age int
	X   int
	Y   int
}

type Snake struct {
	rows      int
	cols      int
	Body      []SnakeCell
	Direction int
	X         int
	Y         int
}

func (s *Snake) Init(rows, cols int) {
	// Initialize the body. Add 1 to the length here because we
	// need to keep the last cell around long enough for the
	// graphics system to clear it.
	s.Body = make([]SnakeCell, SnakeLength+1)

	for i := 0; i < SnakeLength; i++ {
		s.Body[i] = SnakeCell{Age: 0, X: -1, Y: -1}
	}

	s.rows = rows
	s.cols = cols

	s.X = rand.Intn(cols)
	s.Y = rand.Intn(rows)
	s.Direction = rand.Intn(4)

	s.Body[0] = SnakeCell{Age: SnakeLength, X: s.X, Y: s.Y}
}

func (s *Snake) Update() bool {
	// Move snake head
	switch s.Direction {
	case 0: // Up
		s.Y--
	case 1: // Right
		s.X++
	case 2: // Down
		s.Y++
	case 3: // Left
		s.X--
	}

	// Wrap around
	if s.X < 0 {
		s.X = s.cols - 1
	} else if s.X >= s.cols {
		s.X = 0
	}

	if s.Y < 0 {
		s.Y = s.rows - 1
	} else if s.Y >= s.rows {
		s.Y = 0
	}

	// Update snake body
	collision := false

	for i := len(s.Body) - 1; i > 0; i-- {
		s.Body[i] = s.Body[i-1]
		s.Body[i].Age--

		if (s.Body[i].Age > 0) && (s.Body[i].X == s.X) && (s.Body[i].Y == s.Y) {
			// Snake ran into itself
			collision = true
		}
	}
	s.Body[0] = SnakeCell{Age: SnakeLength, X: s.X, Y: s.Y}

	// Randomly change direction
	if rand.Float32() < 0.3 {
		delta := -1

		if rand.Float32() < 0.5 {
			delta = 1
		}

		s.Direction += delta

		if s.Direction < 0 {
			s.Direction = 3
		} else if s.Direction > 3 {
			s.Direction = 0
		}
	}

	return collision
}
