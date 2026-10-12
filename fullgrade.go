package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
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

type Module struct {
	Name    string   `json:"name"`
	Command []string `json:"command"`
	Enabled bool     `json:"enabled"`
}

type Config struct {
	Steps []Module `json:"steps"`
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

	evt.Text = stripANSI(evt.Text)

	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.Encode(evt)

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
	if err := scanner.Err(); err != nil {
		printOutput("error", "Fullgrade", "Error with fullgrade command reader: "+err.Error(), nil)
	}
}

func main() {

	data, err := os.ReadFile("fullconf.json")
	if err != nil {
		log.Fatal(err)
	}

	var conf Config
	if err := json.Unmarshal(data, &conf); err != nil {
		log.Fatal(err)
	}

	printOutput("start", "Fullgrade", "starting full upgrade", nil)

	for _, module := range conf.Steps {
		if len(module.Command) == 0 {
			printOutput("error", module.Name, "no command specified", nil)
			continue
		}
		if module.Enabled == true {
			runModule(module.Name, module.Command[0], module.Command[1:]...)
		}
	}

	exitZero := 0
	printOutput("complete", "Fullgrade", "completed full upgrade", &exitZero)
}
