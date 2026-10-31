// Command glance shows a text one word at a time at a fixed spot, so it reads faster than it
// scrolls: space pauses, h/l go a sentence back and forward, j/k change the speed, f switches to
// big letters, q quits.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"

	"github.com/nyactl/glance/internal/reader"
)

var version = "dev"

const usage = `glance shows a text one word at a time, so it reads faster than it scrolls.

  glance [flags] FILE
  COMMAND | glance [flags]    or glance - to name stdin

Flags:
  -w, --wpm N    start at N words a minute (100-1000; default: the speed of last time)
  -b, --big      big letters
      --fresh    start at the beginning, not where this text was left
  -p, --popup    inside tmux, read in a popup over the current pane
  -v, --version  print the version
  -h, --help     this help

Keys: space pause, h/l sentence back and forward, j/k slower and faster, f big letters, q quit.
Code blocks and tables are shown whole. Settings and where each text was left are kept in
$XDG_STATE_HOME/glance (default ~/.local/state/glance).`

type options struct {
	wpm     int
	big     bool
	fresh   bool
	popup   bool
	version bool
	help    bool
	file    string
}

func parse(args []string) (options, error) {
	var o options
	fs := flag.NewFlagSet("glance", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.IntVar(&o.wpm, "w", 0, "")
	fs.IntVar(&o.wpm, "wpm", 0, "")
	fs.BoolVar(&o.big, "b", false, "")
	fs.BoolVar(&o.big, "big", false, "")
	fs.BoolVar(&o.fresh, "fresh", false, "")
	fs.BoolVar(&o.popup, "p", false, "")
	fs.BoolVar(&o.popup, "popup", false, "")
	fs.BoolVar(&o.version, "v", false, "")
	fs.BoolVar(&o.version, "version", false, "")
	fs.BoolVar(&o.help, "h", false, "")
	fs.BoolVar(&o.help, "help", false, "")
	// Flags may come after the file too: glance notes.md -w 400.
	var files []string
	for {
		if err := fs.Parse(args); err != nil {
			return o, fmt.Errorf("%w\n\n%s", err, usage)
		}
		if args = fs.Args(); len(args) == 0 {
			break
		}
		files, args = append(files, args[0]), args[1:]
	}
	switch {
	case len(files) > 1:
		return o, errors.New("one file at a time\n\n" + usage)
	case len(files) == 1:
		o.file = files[0]
	}
	if o.wpm != 0 && (o.wpm < reader.MinWPM || o.wpm > reader.MaxWPM) {
		return o, fmt.Errorf("--wpm %d: from %d to %d", o.wpm, reader.MinWPM, reader.MaxWPM)
	}
	return o, nil
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		os.Exit(1)
	}
}

func run(args []string) error {
	o, err := parse(args)
	switch {
	case err != nil:
		return waitOnError(err)
	case o.help:
		fmt.Println(usage)
		return nil
	case o.version:
		fmt.Println(version)
		return nil
	}
	stdinTTY := term.IsTerminal(os.Stdin.Fd())
	text, err := input(o.file, stdinTTY)
	if err == nil && strings.TrimSpace(text) == "" {
		err = errors.New("nothing to read")
	}
	if err != nil {
		return waitOnError(err)
	}
	if o.popup && os.Getenv("TMUX") != "" {
		return waitOnError(popup(text, o))
	}

	settings := loadSettings()
	if o.wpm != 0 {
		settings.WPM = o.wpm
	}
	settings.Big = settings.Big || o.big
	m := reader.New(text, settings, saveSettings)
	key := textKey(text)
	if !o.fresh {
		m.Seek(loadPosition(key))
	}
	opts := []tea.ProgramOption{tea.WithAltScreen()}
	// With the text on stdin, the keys come from the terminal itself.
	if !stdinTTY {
		opts = append(opts, tea.WithInputTTY())
	}
	if _, err := tea.NewProgram(m, opts...).Run(); err != nil {
		return waitOnError(err)
	}
	i, done := m.Position()
	savePosition(key, i, done)
	return nil
}

// input is the text to read: a file, or stdin when it is piped or named with -.
func input(file string, stdinTTY bool) (string, error) {
	switch {
	case file != "" && file != "-":
		b, err := os.ReadFile(file)
		return string(b), err
	case file == "" && stdinTTY:
		return "", errors.New("nothing to read: name a file or pipe a text in\n\n" + usage)
	}
	b, err := io.ReadAll(os.Stdin)
	return string(b), err
}

// popup reads text in a tmux popup over the current pane. The popup gets the text as a file of its
// own, removed when it closes, since stdin does not reach it.
func popup(text string, o options) error {
	f, err := os.CreateTemp("", "glance-*.txt")
	if err != nil {
		return err
	}
	if _, err := f.WriteString(text); err != nil {
		f.Close()
		os.Remove(f.Name())
		return err
	}
	f.Close()
	self, err := os.Executable()
	if err != nil {
		self = "glance"
	}
	cmd := []string{"GLANCE_POPUP=1", quote(self)}
	if o.wpm != 0 {
		cmd = append(cmd, "--wpm", strconv.Itoa(o.wpm))
	}
	if o.big {
		cmd = append(cmd, "--big")
	}
	if o.fresh {
		cmd = append(cmd, "--fresh")
	}
	cmd = append(cmd, quote(f.Name()))
	script := strings.Join(cmd, " ") + "; rm -f " + quote(f.Name())
	out, err := exec.Command("tmux", "display-popup", "-E", "-w", "80%", "-h", "60%", script).CombinedOutput()
	if err != nil {
		os.Remove(f.Name())
		return fmt.Errorf("tmux popup: %v %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// waitOnError shows an error until a key is pressed: in a tmux popup that closes when glance ends,
// it would be gone before it could be read. GLANCE_POPUP=1 marks such a popup.
func waitOnError(err error) error {
	if err == nil {
		return nil
	}
	fmt.Fprintf(os.Stderr, "glance: %v\n", err)
	if !term.IsTerminal(os.Stderr.Fd()) || os.Getenv("GLANCE_POPUP") == "" {
		return err
	}
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
