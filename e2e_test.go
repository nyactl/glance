package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
)

// The end-to-end tests run the built binary the way a user does: in a terminal, or with a pipe.

// go test sets GOCOVERDIR for itself, so the binary's coverage goes to a variable of its own.
var coverDir = os.Getenv("GLANCE_COVERDIR")

// environ is the environment of a run of glance, with its state in dir.
func environ(dir string) []string {
	env := append(os.Environ(), "XDG_STATE_HOME="+dir, "TERM=xterm-256color")
	if coverDir != "" {
		env = append(env, "GOCOVERDIR="+coverDir)
	}
	return env
}

var (
	buildOnce sync.Once
	binPath   string
	buildErr  error
)

func binary(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("end-to-end")
	}
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "glance-e2e")
		if err != nil {
			buildErr = err
			return
		}
		binPath = filepath.Join(dir, "glance")
		args := []string{"build", "-ldflags", "-X main.version=9.9.9", "-o", binPath}
		// make cover counts what the runs of the binary cover too.
		if coverDir != "" {
			args = append(args, "-cover", "-coverpkg=./...")
		}
		out, err := exec.Command("go", append(args, ".")...).CombinedOutput()
		if err != nil {
			buildErr = errors.New(string(out))
		}
	})
	if buildErr != nil {
		t.Fatalf("build: %v", buildErr)
	}
	return binPath
}

// glance runs the binary without a terminal and returns its output and exit code.
func glance(t *testing.T, stdin string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(binary(t), args...)
	cmd.Env = environ(t.TempDir())
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return string(out), code
}

func TestE2EVersionAndHelp(t *testing.T) {
	if out, code := glance(t, "", "--version"); code != 0 || strings.TrimSpace(out) != "9.9.9" {
		t.Fatalf("--version: %d %q", code, out)
	}
	for _, flag := range []string{"-h", "--help"} {
		if out, code := glance(t, "", flag); code != 0 || !strings.Contains(out, "glance [flags] FILE") {
			t.Fatalf("%s: %d %q", flag, code, out)
		}
	}
}

func TestE2EErrors(t *testing.T) {
	cases := map[string]struct {
		stdin string
		args  []string
		want  string
	}{
		"missing file": {args: []string{filepath.Join(t.TempDir(), "nope.md")}, want: "no such file"},
		"empty stdin":  {stdin: " \n\t\n", want: "nothing to read"},
		"two files":    {args: []string{"a", "b"}, want: "one file"},
		"unknown flag": {args: []string{"--nope"}, want: "nope"},
		"bad speed":    {args: []string{"-w", "20", "x.md"}, want: "from 100 to 1000"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			out, code := glance(t, c.stdin, c.args...)
			if code == 0 || !strings.Contains(out, c.want) {
				t.Fatalf("%d %q, want %q", code, out, c.want)
			}
		})
	}
}

// session is glance running in a pseudo terminal.
type session struct {
	t     *testing.T
	pty   *os.File
	cmd   *exec.Cmd
	mu    sync.Mutex
	out   bytes.Buffer
	done  chan error
	state string
}

// start runs glance in a terminal of its own, with stdin from the terminal unless given.
func start(t *testing.T, stdin io.Reader, args ...string) *session {
	return startIn(t, t.TempDir(), stdin, args...)
}

// startIn is start with the state kept in dir, to see what a run before left.
func startIn(t *testing.T, dir string, stdin io.Reader, args ...string) *session {
	t.Helper()
	s := &session{t: t, cmd: exec.Command(binary(t), args...), done: make(chan error, 1), state: dir}
	s.cmd.Env = environ(dir)
	var tty *os.File
	var err error
	s.pty, tty, err = pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	pty.Setsize(s.pty, &pty.Winsize{Rows: 24, Cols: 100})
	s.cmd.Stdout, s.cmd.Stderr = tty, tty
	s.cmd.Stdin = tty
	if stdin != nil {
		s.cmd.Stdin = stdin
	}
	// The terminal becomes glance's own, so /dev/tty finds it when the text comes from a pipe.
	s.cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 1}
	if err := s.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	tty.Close()
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := s.pty.Read(buf)
			s.mu.Lock()
			s.out.Write(buf[:n])
			s.mu.Unlock()
			// Answer what the terminal is asked, the background colour and the cursor, as a real
			// one would: without an answer the colour detection waits.
			if bytes.Contains(buf[:n], []byte("\x1b]11;?")) {
				s.pty.Write([]byte("\x1b]11;rgb:0000/0000/0000\x1b\\"))
			}
			if bytes.Contains(buf[:n], []byte("\x1b[6n")) {
				s.pty.Write([]byte("\x1b[1;1R"))
			}
			if err != nil {
				return
			}
		}
	}()
	go func() { s.done <- s.cmd.Wait() }()
	t.Cleanup(func() {
		s.cmd.Process.Kill()
		s.pty.Close()
	})
	return s
}

func (s *session) send(keys string) {
	s.t.Helper()
	if _, err := s.pty.Write([]byte(keys)); err != nil {
		s.t.Fatal(err)
	}
}

// waitFor waits until the screen output contains want.
func (s *session) waitFor(want string) {
	s.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		got := s.out.String()
		s.mu.Unlock()
		if strings.Contains(got, want) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.t.Fatalf("waiting for %q, got:\n%q", want, s.out.String())
}

func (s *session) quit() {
	s.t.Helper()
	s.send("q")
	if err := s.exit(); err != nil {
		s.t.Fatalf("exit: %v", err)
	}
}

func (s *session) exit() error {
	s.t.Helper()
	select {
	case err := <-s.done:
		return err
	case <-time.After(5 * time.Second):
		s.t.Fatal("glance did not exit")
		return nil
	}
}

func (s *session) settings() map[string]any {
	s.t.Helper()
	b, err := os.ReadFile(filepath.Join(s.state, "glance", "settings.json"))
	if err != nil {
		s.t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		s.t.Fatal(err)
	}
	return m
}

func writeText(t *testing.T, text string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "notes.md")
	os.WriteFile(p, []byte(text), 0o600)
	return p
}

const story = "Glance starts here. The second sentence is a little longer than the first one. " +
	"The third sentence ends the paragraph.\n\nNew paragraph now. And it ends."

func TestE2EReadsAFileAndKeepsTheSpeed(t *testing.T) {
	s := start(t, nil, writeText(t, story))
	s.waitFor("wpm")
	s.send(" ")
	s.waitFor("paused")
	s.send("kk")
	s.waitFor("400 wpm")
	s.quit()
	if got := s.settings()["wpm"]; got != float64(400) {
		t.Fatalf("kept speed: %v", got)
	}
}

func TestE2EReadsAPipeWithKeysFromTheTerminal(t *testing.T) {
	s := start(t, strings.NewReader("Piped text is read while the keys come from the terminal."))
	s.waitFor("wpm")
	s.send(" ")
	s.waitFor("paused")
	s.quit()
}

func TestE2EFlagsSetTheSpeedAndSize(t *testing.T) {
	s := start(t, nil, "--wpm", "500", "--big", writeText(t, story))
	s.waitFor("500 wpm")
	// The big font draws letters with characters like these.
	s.waitFor("888")
	s.send(" ")
	s.waitFor("paused")
	s.quit()
	// Flags are for this run; only the keys change what is kept.
	if _, err := os.Stat(filepath.Join(s.state, "glance", "settings.json")); !os.IsNotExist(err) {
		t.Fatal("flags were saved as settings")
	}
}

func TestE2EPauseShowsTheSentence(t *testing.T) {
	s := start(t, nil, "-w", "100", writeText(t, story))
	s.waitFor("wpm")
	s.send(" ")
	s.waitFor("paused")
	s.waitFor("starts")
	s.waitFor("here.")
	s.quit()
}

func TestE2EResumesWhereTheTextWasLeft(t *testing.T) {
	p := writeText(t, story)
	s := start(t, nil, "-w", "100", p)
	s.waitFor("wpm")
	s.send(" ")
	s.waitFor("paused")
	s.send("lll")
	s.waitFor("N\x1b[1;31me")
	s.quit()

	resumed := startIn(t, s.state, nil, "-w", "100", p)
	resumed.waitFor("N\x1b[1;31me")
	resumed.quit()

	fresh := startIn(t, s.state, nil, "-w", "100", "--fresh", p)
	fresh.waitFor("Gl\x1b[1;31ma")
	fresh.quit()
}

func TestE2ENoTextInATerminal(t *testing.T) {
	s := start(t, nil)
	s.waitFor("nothing to read")
	if err := s.exit(); err == nil {
		t.Fatal("exit 0 with nothing to read")
	}
}

func TestE2ENoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	s := start(t, nil, "-w", "100", writeText(t, story))
	s.waitFor("Glance")
	s.send(" ")
	s.waitFor("paused")
	s.quit()
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.Contains(s.out.String(), "31m") {
		t.Fatal("colour with NO_COLOR set")
	}
}
