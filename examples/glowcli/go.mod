module glowcli

go 1.21

replace github.com/rpi-ws281x/rpi-ws281x-go/pkg/whisper => ../../pkg/whisper

replace github.com/rpi-ws281x/rpi-ws281x-go/pkg/glowy => ../../pkg/glowy

require github.com/rpi-ws281x/rpi-ws281x-go/pkg/whisper v0.0.0

require github.com/rpi-ws281x/rpi-ws281x-go/pkg/glowy v0.0.0
