// Command glance shows a text one word at a time at a fixed spot, so it reads faster than it
// scrolls: space pauses, h/l go a sentence back and forward, j/k change the speed, f switches to
// big letters, q quits.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"

	"github.com/nyactl/glance/internal/reader"
)

const usage = `glance shows a text one word at a time: space pause, h/l sentence back and forward,
j/k slower and faster, f big letters, q quit. Code blocks and tables are shown whole.

  glance FILE
  glance -          the text from stdin, like pbpaste | glance
  COMMAND | glance  the same`

func main() {
	if err := run(os.Args[1:]); err != nil {
		os.Exit(1)
	}
}

func run(args []string) error {
	text, err := input(args)
	if err == nil && strings.TrimSpace(text) == "" {
		err = errors.New("nothing to read")
	}
	if err != nil {
		return waitOnError(err)
	}
	opts := []tea.ProgramOption{tea.WithAltScreen()}
	// With the text on stdin, the keys come from the terminal itself.
	if !term.IsTerminal(os.Stdin.Fd()) {
		opts = append(opts, tea.WithInputTTY())
	}
	m := reader.New(text, loadSettings(), saveSettings)
	_, err = tea.NewProgram(m, opts...).Run()
	return err
}

// input is the text to read: a file, or stdin when it is piped or named with -.
func input(args []string) (string, error) {
	switch {
	case len(args) == 1 && (args[0] == "-h" || args[0] == "--help"):
		fmt.Println(usage)
		os.Exit(0)
	case len(args) == 1 && args[0] != "-":
		b, err := os.ReadFile(args[0])
		return string(b), err
	case len(args) > 1, len(args) == 0 && term.IsTerminal(os.Stdin.Fd()):
		return "", errors.New("usage:\n" + usage)
	}
	b, err := io.ReadAll(os.Stdin)
	return string(b), err
}

// waitOnError shows an error until a key is pressed: in a tmux popup that closes when glance ends,
// it would be gone before it could be read.
func waitOnError(err error) error {
	fmt.Fprintf(os.Stderr, "glance: %v\n", err)
	tty, terr := os.Open("/dev/tty")
	if terr != nil {
		return err
	}
	defer tty.Close()
	fmt.Fprint(os.Stderr, "\nany key closes")
	if old, rerr := term.MakeRaw(tty.Fd()); rerr == nil {
		tty.Read(make([]byte, 1))
		term.Restore(tty.Fd(), old)
	}
	fmt.Fprintln(os.Stderr)
	return err
}

func settingsPath() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "glance", "settings.json")
}

func loadSettings() reader.Settings {
	var s reader.Settings
	if b, err := os.ReadFile(settingsPath()); err == nil {
		json.Unmarshal(b, &s)
	}
	return s
}

func saveSettings(s reader.Settings) {
	b, err := json.Marshal(s)
	if err != nil {
		return
	}
	if os.MkdirAll(filepath.Dir(settingsPath()), 0o700) == nil {
		os.WriteFile(settingsPath(), b, 0o600)
	}
}
