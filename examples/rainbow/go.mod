module rainbow

go 1.20

require github.com/rpi-ws281x/rpi-ws281x-go/pkg/ws2811 v0.0.0

replace github.com/rpi-ws281x/rpi-ws281x-go/pkg/ws2811 => ../../pkg/ws2811

require (
	github.com/pkg/errors v0.9.1 // indirect
	github.com/stretchr/testify v1.11.1 // indirect
)
