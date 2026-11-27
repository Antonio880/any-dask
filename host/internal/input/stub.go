//go:build !windows && !darwin

package input

type stubInjector struct{}

func NewInjector() Injector { return &stubInjector{} }

func (s *stubInjector) ScreenBounds() ScreenBounds                   { return ScreenBounds{Width: 1920, Height: 1080} }
func (s *stubInjector) MoveRelative(dx, dy int) error                { return nil }
func (s *stubInjector) MoveAbsolute(x, y int) error                  { return nil }
func (s *stubInjector) MouseButton(button uint8, action uint8) error { return nil }
func (s *stubInjector) Scroll(dx, dy int) error                      { return nil }
func (s *stubInjector) KeyEvent(key, action string, m []string) error { return nil }
func (s *stubInjector) TypeText(text string) error                   { return nil }
func (s *stubInjector) Hotkey(keys []string) error                   { return nil }
func (s *stubInjector) Drag(x1, y1, x2, y2 int) error                { return nil }
