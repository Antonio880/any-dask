//go:build !windows && !darwin

package osctx

func ActiveWindowTitle() string    { return "" }
func SetClipboard(text string) error { return nil }
