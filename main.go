package main

import (
	"fmt"
	"log"
	"os"
	"runtime/debug"

	"github.com/johnthethird/thresher/commands"
	"github.com/johnthethird/thresher/ulimit"
	"github.com/johnthethird/thresher/version"
)

func panicHandler() {
	if panicPayload := recover(); panicPayload != nil {
		stack := string(debug.Stack())
		fmt.Fprintln(os.Stderr, "================================================================================")
		fmt.Fprintln(os.Stderr, "Thresher Wallet encountered a fatal error. This is a bug!")
		fmt.Fprintln(os.Stderr, "================================================================================")
		fmt.Fprintf(os.Stderr, "STT Wallet Version: %s\n", version.Version)
		fmt.Fprintf(os.Stderr, "Build Date:        %s\n", version.BuildDate)
		fmt.Fprintf(os.Stderr, "Git Commit:        %s\n", version.GitCommit)
		fmt.Fprintf(os.Stderr, "Go Version:        %s\n", version.GoVersion)
		fmt.Fprintf(os.Stderr, "OS / Arch:         %s\n", version.OsArch)
		fmt.Fprintf(os.Stderr, "Panic:             %s\n\n", panicPayload)
		fmt.Fprintln(os.Stderr, stack)
		os.Exit(1)
	}
}

func main() {
	defer panicHandler()	

	// The TUI requires lots of open files, so raise it here
	if err := ulimit.Set(ulimit.DefaultFDLimit); err != nil {
		log.Fatal(fmt.Errorf("failed to set fd limit correctly due to: %w", err))
	}

	commands.Execute()
}
