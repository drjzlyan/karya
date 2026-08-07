package term

// Event is a decoded terminal input event: a key press, a mouse action, a
// bracketed paste, or a resize. It is delivered into the tui.Program message
// stream.
type Event interface{ isEvent() }

// KeyEvent is a single key press.
type KeyEvent struct{ Key Key }

func (KeyEvent) isEvent() {}

// PasteEvent carries text delivered via bracketed paste as one unit, so a large
// paste is not misread as thousands of key presses.
type PasteEvent struct{ Text string }

func (PasteEvent) isEvent() {}

// ResizeEvent reports a new terminal size (in cells).
type ResizeEvent struct{ Cols, Rows int }

func (ResizeEvent) isEvent() {}

// MouseButton identifies which button a MouseEvent concerns.
type MouseButton uint8

// Mouse buttons.
const (
	MouseNone MouseButton = iota
	MouseLeft
	MouseMiddle
	MouseRight
	MouseWheelUp
	MouseWheelDown
)

// MouseAction is what happened to the button.
type MouseAction uint8

// Mouse actions.
const (
	MousePress MouseAction = iota
	MouseRelease
	MouseMotion
)

// MouseEvent is a mouse action at a cell position (0-based).
type MouseEvent struct {
	X, Y   int
	Button MouseButton
	Action MouseAction
	Mod    Mod
}

func (MouseEvent) isEvent() {}
