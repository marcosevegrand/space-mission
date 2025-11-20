package mothership

import (
	_ "embed"
)

//go:embed frontend.html
var FrontendHTML []byte

// GetFrontendHTML returns the embedded frontend HTML as a string.
func GetFrontendHTML() string {
	return string(FrontendHTML)
}
