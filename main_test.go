package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nyactl/glance/internal/reader"
)

func TestParse(t *testing.T) {
	cases := []struct {
		args []string
		want options
		err  string
	}{
		{args: nil},
		{args: []string{"notes.md"}, want: options{file: "notes.md"}},
		{args: []string{"-"}, want: options{file: "-"}},
		{args: []string{"-w", "500", "-b", "notes.md"}, want: options{wpm: 500, big: true, file: "notes.md"}},
		{args: []string{"--wpm=250", "--big", "--fresh", "--popup"}, want: options{wpm: 250, big: true, fresh: true, popup: true}},
		{args: []string{"-v"}, want: options{version: true}},
		{args: []string{"--help"}, want: options{help: true}},
		{args: []string{"notes.md", "-w", "400", "--fresh"}, want: options{wpm: 400, fresh: true, file: "notes.md"}},
		{args: []string{"--", "-w"}, want: options{file: "-w"}},
		{args: []string{"a", "b"}, err: "one file"},
		{args: []string{"a", "-b", "b"}, err: "one file"},
		{args: []string{"--wpm", "5000"}, err: "from 100 to 1000"},
		{args: []string{"--wpm", "fast"}, err: "invalid value"},
		{args: []string{"--nope"}, err: "not defined"},
	}
	for _, c := range cases {
		t.Run(strings.Join(c.args, " "), func(t *testing.T) {
			got, err := parse(c.args)
			if c.err != "" {
				if err == nil || !strings.Contains(err.Error(), c.err) {
					t.Fatalf("error %v, want %q", err, c.err)
				}
				return
			}
			if err != nil || got != c.want {
				t.Fatalf("%+v %v, want %+v", got, err, c.want)
			}
		})
	}
}

func withStdin(t *testing.T, text string) {
	t.Helper()
	r, w, _ := os.Pipe()
	w.WriteString(text)
	w.Close()
	stdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = stdin })
}

func TestInputFromAFileOrStdin(t *testing.T) {
	p := filepath.Join(t.TempDir(), "notes.md")
	os.WriteFile(p, []byte("from the file"), 0o600)
	if got, err := input(p, true); err != nil || got != "from the file" {
		t.Fatalf("file: %q %v", got, err)
	}
	if _, err := input(filepath.Join(t.TempDir(), "missing"), true); err == nil {
		t.Fatal("a missing file read")
	}
	if _, err := input("", true); err == nil || !strings.Contains(err.Error(), "nothing to read") {
		t.Fatalf("a terminal on stdin: %v", err)
	}
	withStdin(t, "piped in")
	if got, err := input("", false); err != nil || got != "piped in" {
		t.Fatalf("stdin: %q %v", got, err)
	}
	withStdin(t, "named with -")
	if got, err := input("-", true); err != nil || got != "named with -" {
		t.Fatalf("-: %q %v", got, err)
	}
}

func TestSettingsAreKept(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if s := loadSettings(); s != (reader.Settings{}) {
		t.Fatalf("first run: %+v", s)
	}
	saveSettings(reader.Settings{WPM: 425, Big: true})
	if s := loadSettings(); s != (reader.Settings{WPM: 425, Big: true}) {
		t.Fatalf("kept: %+v", s)
	}
	os.WriteFile(filepath.Join(stateDir(), "settings.json"), []byte("{broken"), 0o600)
	if s := loadSettings(); s != (reader.Settings{}) {
		t.Fatalf("a broken file: %+v", s)
	}
}

func TestStateDirDefaultsToLocalState(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "/home/someone")
	if got := stateDir(); got != "/home/someone/.local/state/glance" {
		t.Fatal(got)
	}
}

func TestPositionsAreKeptPerText(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	a, b := textKey("first text"), textKey("second text")
	if a == b || textKey("first text") != a {
		t.Fatal("keys")
	}
	if loadPosition(a) != 0 {
		t.Fatal("a new text starts at 0")
	}
	savePosition(a, 42, false)
	savePosition(b, 7, false)
	if loadPosition(a) != 42 || loadPosition(b) != 7 {
		t.Fatalf("kept: %d %d", loadPosition(a), loadPosition(b))
	}
	savePosition(a, 99, true)
	if loadPosition(a) != 0 || loadPosition(b) != 7 {
		t.Fatal("a text read to its end is forgotten")
	}
	savePosition(b, 0, false)
	if loadPosition(b) != 0 {
		t.Fatal("going back to the start forgets the text")
	}
}

func TestPositionsAreBounded(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	for i := range maxPositions + 20 {
		savePosition(textKey(fmt.Sprint(i)), i+1, false)
	}
	ps := map[string]position{}
	readJSON("positions.json", &ps)
	if len(ps) != maxPositions {
		t.Fatalf("%d positions", len(ps))
	}
}

func TestQuote(t *testing.T) {
	for in, want := range map[string]string{"plain": "'plain'", "it's": `'it'\''s'`, "a b": "'a b'"} {
		if got := quote(in); got != want {
			t.Errorf("%s: %s", in, got)
		}
	}
	out, err := exec.Command("sh", "-c", "printf %s "+quote(`a 'b' $c "d"`)).Output()
	if err != nil || string(out) != `a 'b' $c "d"` {
		t.Fatalf("the shell reads it back as %q %v", out, err)
	}
}

// A fake tmux on PATH records what glance asks it and runs the popup's command, as tmux would.
func TestPopupRunsGlanceInTmuxAndRemovesTheText(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "tmux.log")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + quote(log) + "\neval \"$(eval echo \\${$#})\" >/dev/null\n"
	os.WriteFile(filepath.Join(dir, "tmux"), []byte(script), 0o755)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := popup("A text for the popup.", options{wpm: 300, big: true, fresh: true}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(log)
	args := strings.Split(strings.TrimSpace(string(b)), "\n")
	if args[0] != "display-popup" || args[1] != "-E" {
		t.Fatalf("tmux %q", args)
	}
	cmd := args[len(args)-1]
	for _, want := range []string{"GLANCE_POPUP=1", "--wpm 300", "--big", "--fresh", "; rm -f '"} {
		if !strings.Contains(cmd, want) {
			t.Fatalf("popup command %q lacks %q", cmd, want)
		}
	}
	file := cmd[strings.LastIndex(cmd, "rm -f ")+len("rm -f "):]
	file = strings.Trim(file, "'")
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("the text was left behind: %s", file)
	}
}

func TestPopupFailsWithoutTmux(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if err := popup("text", options{}); err == nil || !strings.Contains(err.Error(), "tmux popup") {
		t.Fatalf("%v", err)
	}
}
