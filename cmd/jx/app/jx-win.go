// +build windows

package app

import (
	"os"
	"syscall"

	"github.com/jenkins-x/jx/pkg/cmd"
	"github.com/jenkins-x/jx/pkg/cmd/clients"
)

// Run runs the command, if args are not nil they will be set on the command
func Run(args []string) error {
	configureTerminalForAnsiEscapes()
	cmd := cmd.NewJXCommand(clients.NewFactory(), os.Stdin, os.Stdout, os.Stderr, nil)
	if len(args) > 0 {
		args = args[1:]
		cmd.SetArgs(args)
	}
	return cmd.Execute()
}

const (
	// https://docs.microsoft.com/en-us/windows/console/setconsolemode
	enableProcessedOutput           = 0x1
	enableWrapAtEOLOutput           = 0x2
	enableVirtualTerminalProcessing = 0x4
)

// configureTerminalForAnsiEscapes enables the windows 10 console to translate ansi escape sequences
// requires windows 10 1511 or higher and fails gracefully on older versions (and prior releases like windows 7)
// https://docs.microsoft.com/en-us/windows/console/console-virtual-terminal-sequences
func configureTerminalForAnsiEscapes() {

	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	kern32SetConsoleMode := kernel32.NewProc("SetConsoleMode")

	// stderr
	handle := syscall.Handle(os.Stderr.Fd())
	kern32SetConsoleMode.Call(uintptr(handle), enableProcessedOutput|enableWrapAtEOLOutput|enableVirtualTerminalProcessing)

	// stdout
	handle = syscall.Handle(os.Stdout.Fd())
	kern32SetConsoleMode.Call(uintptr(handle), enableProcessedOutput|enableWrapAtEOLOutput|enableVirtualTerminalProcessing)
}
