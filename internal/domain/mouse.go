package domain

// MouseReport is which mouse events a pane's program has asked for.
type MouseReport int

const (
	// MouseReportOff means the program does not use the mouse.
	MouseReportOff MouseReport = iota
	// MouseReportClicks asks for presses, releases and the wheel.
	MouseReportClicks
	// MouseReportDrags also asks for movement while a button is held.
	MouseReportDrags
	// MouseReportMotion asks for all movement.
	MouseReportMotion
)

// MouseAction is what happened to the mouse.
type MouseAction int

const (
	MousePress MouseAction = iota
	MouseRelease
	MouseMove
	MouseWheel
)

// MouseButton is the button involved in a mouse event.
type MouseButton int

const (
	ButtonNone MouseButton = iota
	ButtonLeft
	ButtonMiddle
	ButtonRight
	WheelUp
	WheelDown
)

// MouseEvent is a mouse event for a pane, at a cell of its screen.
type MouseEvent struct {
	Action MouseAction
	Button MouseButton
	X, Y   int
	Mod    KeyMod
}
