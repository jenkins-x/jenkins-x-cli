package main

import (
	"os"

	"github.com/jenkins-x/jx/cmd/jx/app"
)

// Entrypoint for jx command
func main() {
	if err := app.Run(nil); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
