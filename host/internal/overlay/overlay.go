package overlay

// State controls the visual intensity and behavior of the Desktop Liquid Glass overlay.
type State string

const (
	StateHidden     State = "HIDDEN"
	StateConnected  State = "CONNECTED"
	StateAIThinking State = "THINKING"
	StateAIActing   State = "ACTING"
)

// Overlay renders a click-through iOS Liquid Glass / Apple Intelligence visual layer
// directly on the controlled Windows 11 or macOS monitor.
type Overlay interface {
	SetEnabled(enabled bool)
	IsEnabled() bool
	SetState(state State, label string)
	TriggerRipple(x, y int, isAI bool)
	Close()
}
