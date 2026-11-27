//go:build !windows && !darwin

package overlay

type noopOverlay struct{ enabled bool }

func RunMainLoop(serverFn func()) { serverFn() }
func New(enabled bool) Overlay    { return &noopOverlay{enabled: enabled} }

func (n *noopOverlay) SetEnabled(e bool)            { n.enabled = e }
func (n *noopOverlay) IsEnabled() bool              { return n.enabled }
func (n *noopOverlay) SetState(s State, l string)                {}
func (n *noopOverlay) SetBounds(displayIndex, x, y, w, h int)    {}
func (n *noopOverlay) TriggerRipple(x, y int, a bool)            {}
func (n *noopOverlay) Close()                       {}
