// physick-cli gives one character the Flask of Wondrous Physick and sets the
// "obtained flask" event flag, headless (no Wails UI). See PhysickCLIMain.
package main

import (
	"os"

	"github.com/oisis/EldenRing-SaveForge/internal/application"
)

func main() {
	os.Exit(application.PhysickCLIMain(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
