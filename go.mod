module github.com/unxed/vtui

go 1.24.0

require (
	github.com/mattn/go-runewidth v0.0.15
	github.com/unxed/vtinput v0.0.0
	golang.org/x/term v0.40.0
)

require (
	github.com/rivo/uniseg v0.2.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
)

// This line tells Go to use a local copy of vtinput
replace github.com/unxed/vtinput => ../vtinput
