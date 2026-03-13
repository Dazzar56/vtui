module github.com/unxed/vtui

go 1.24.0

require (
	github.com/unxed/vtinput v0.0.0
	golang.org/x/term v0.40.0
)

// Эта строка указывает Go использовать локальную копию vtinput
replace github.com/unxed/vtinput => ../vtinput