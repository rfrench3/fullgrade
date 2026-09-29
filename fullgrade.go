package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sync"
)

// JSON used as output for easy parsing.
type Event struct {
	Event    string `json:"event"`
	Command  string `json:"command"`
	Text     string `json:"text,omitempty"`
	ExitCode *int   `json:"exit_code,omitempty"`
}

// removes ANSI escape codes from a string, relevant for distrobox.
func stripANSI(s string) string {
	var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]`)
	return ansiRegex.ReplaceAllString(s, "")
}

func printOutput(event string, command string, text string, exitCode *int) {
	evt := Event{
		Event:    event,
		Command:  command,
		Text:     text,
		ExitCode: exitCode,
	}
	data, err := json.Marshal(evt)
	if err == nil {
		fmt.Println(string(data))
	}
}

func runModule(commandName string, name string, args ...string) int {
	cmd := exec.Command(name, args...)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating stdout pipe: %v\n", err)
		return 1
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating stderr pipe: %v\n", err)
		return 1
	}

	printOutput("start", commandName, "", nil)

	if err := cmd.Start(); err != nil {
		printOutput("error", commandName, err.Error(), nil)
		code := 1
		printOutput("complete", commandName, "", &code)
		return code
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Stream stdout line-by-line
	go func() {
		defer wg.Done()
		streamReader(stdoutPipe, commandName, "output")
	}()

	// Stream stderr line-by-line
	go func() {
		defer wg.Done()
		streamReader(stderrPipe, commandName, "error")
	}()

	// Wait for readers to read all pipe output before waiting on command completion
	wg.Wait()

	exitCode := 0
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	printOutput("complete", commandName, "", &exitCode)
	return exitCode
}

// streamReader reads from a pipe line-by-line and emits JSON events.
func streamReader(r io.Reader, commandName string, eventType string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		printOutput(eventType, commandName, scanner.Text(), nil)
	}
}

func main() {
	printOutput("start", "Fullgrade", "starting full upgrade", nil)

	runModule("System", "rpm-ostree", "upgrade", "--unchanged-exit-77")
	runModule("Brew", "bash", "-c", "brew update && brew upgrade -y")
	runModule("Flatpak", "flatpak", "update", "-y")
	runModule("Distrobox", "distrobox-upgrade", "--all")

	exitZero := 0
	printOutput("complete", "Fullgrade", "completed full upgrade", &exitZero)
}
