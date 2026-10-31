package reader

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

const sample = "## Result\n\nThe **parser** is ported. See [the PR](https://x/1), it took `go 1.23`.\n\n" +
	"```go\nfunc main() {}\n```\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n- first item\n- second, longer item"

func words(cs []chunk) string {
	var out []string
	for _, c := range cs {
		w := c.word
		if c.block != "" {
			w = "[" + c.block + "]"
		}
		if c.start {
			w = "^" + w
		}
		out = append(out, w)
	}
	return strings.Join(out, " ")
}

func TestSplit(t *testing.T) {
	cs := split(sample)
	want := "^Result ^The parser is ported. ^See the PR, it took go 1.23. ^[func main() {}] ^[| a | b |\n|---|---|\n| 1 | 2 |] ^first item ^second, longer item"
	if got := words(cs); got != want {
		t.Fatalf("split:\n%s\nwant\n%s", got, want)
	}
	if cs[4].weight != 2.2 || cs[7].weight != 1.5 || cs[1].weight != 1 || cs[0].weight < 2.5 {
		t.Fatalf("weights: %+v", cs[:8])
	}
	if split("") != nil || len(split("```\nonly code")) != 1 {
		t.Fatal("empty or open code block")
	}
}

func TestFocus(t *testing.T) {
	for w, want := range map[string]int{"a": 0, "word": 1, "reading": 2, "comprehension": 3, "internationalization": 4} {
		if got := focus(w); got != want {
			t.Errorf("%s: %d", w, got)
		}
	}
}

func key(s string) tea.KeyMsg {
	switch s {
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestPlayPauseAndSentences(t *testing.T) {
	var saved int
	m := New("One two three. Four five. Six.", Settings{WPM: 300}, func(s Settings) { saved = s.WPM })
	if m.paused || m.Init() == nil {
		t.Fatal("does not start playing")
	}
	m.Update(tickMsg{gen: m.gen})
	m.Update(tickMsg{gen: m.gen})
	if m.chunks[m.i].word != "three." {
		t.Fatalf("at %q", m.chunks[m.i].word)
	}
	// An old timer does not move on.
	old := m.gen
	m.Update(key("l"))
	if m.chunks[m.i].word != "Four" {
		t.Fatalf("l: %q", m.chunks[m.i].word)
	}
	m.Update(tickMsg{gen: old})
	if m.chunks[m.i].word != "Four" {
		t.Fatal("an old tick moved on")
	}
	m.Update(tickMsg{gen: m.gen})
	m.Update(key("h"))
	if m.chunks[m.i].word != "Four" {
		t.Fatalf("h to the start of this sentence: %q", m.chunks[m.i].word)
	}
	m.Update(key("h"))
	if m.chunks[m.i].word != "One" {
		t.Fatalf("h again: %q", m.chunks[m.i].word)
	}
	m.Update(key(" "))
	if !m.paused || !strings.Contains(m.View(), "paused") {
		t.Fatal("space does not pause")
	}
	m.Update(tickMsg{gen: m.gen})
	if m.chunks[m.i].word != "One" {
		t.Fatal("moved while paused")
	}
	m.Update(key("k"))
	m.Update(key("k"))
	m.Update(key("j"))
	if m.wpm != 325 || saved != 325 {
		t.Fatalf("speed: %d %d", m.wpm, saved)
	}
	for range 50 {
		m.Update(key("k"))
	}
	if m.wpm != MaxWPM {
		t.Fatalf("max: %d", m.wpm)
	}
	// Played to the end it stops, and space starts again.
	m.Update(key(" "))
	for range 10 {
		m.Update(tickMsg{gen: m.gen})
	}
	if !m.paused || m.chunks[m.i].word != "Six." || !strings.Contains(m.View(), "the end") {
		t.Fatalf("end: %v %q", m.paused, m.chunks[m.i].word)
	}
	m.Update(key(" "))
	if m.paused || m.i != 0 {
		t.Fatal("space at the end does not start over")
	}
	if _, cmd := m.Update(key("q")); cmd == nil {
		t.Fatal("q does not quit")
	}
}

func TestBlocksPause(t *testing.T) {
	m := New("Before it.\n\n```\ncode here\n```\n\nAfter it.", Settings{WPM: 300}, nil)
	m.Update(tickMsg{gen: m.gen})
	m.Update(tickMsg{gen: m.gen})
	if !m.paused || !strings.Contains(m.View(), "code here") || !strings.Contains(m.View(), "space goes on") {
		t.Fatalf("block:\n%s", m.View())
	}
	m.Update(key(" "))
	if m.paused || m.chunks[m.i].word != "After" {
		t.Fatalf("after the block: %v %q", m.paused, m.chunks[m.i].word)
	}
	if New("```\nx\n```", Settings{WPM: 300}, nil).paused != true {
		t.Fatal("a text starting with a block should wait")
	}
}

func TestViewPutsTheFocusLetterAtTheSameColumn(t *testing.T) {
	col := func(word string) int {
		m := New(word, Settings{WPM: 300}, nil)
		m.width, m.height = 60, 9
		for _, l := range strings.Split(m.View(), "\n") {
			if strings.Contains(l, word[1:]) {
				return strings.Index(l, word[:focus(word)]) + focus(word)
			}
		}
		return -1
	}
	if a, b := col("reading"), col("internationalization"); a != b || a != 20 {
		t.Fatalf("focus columns: %d %d", a, b)
	}
}

func TestBigLetters(t *testing.T) {
	rows, from, to, ok := bigWord("Wort")
	if !ok || len(rows) < 8 || from <= 0 || to <= from {
		t.Fatalf("big: %v %d %d %q", ok, from, to, rows)
	}
	// The focus letter, o, is drawn between from and to.
	o, _, _, _ := bigWord("o")
	for i, r := range rows {
		if i < len(o) && strings.TrimRight(r[from:min(to, len(r))], " ") != strings.TrimRight(o[i], " ") {
			t.Fatalf("row %d: %q is not %q", i, r[from:min(to, len(r))], o[i])
		}
	}
	if _, _, _, ok := bigWord("Größe"); !ok {
		t.Fatal("umlauts are spelled out")
	}
	if _, _, _, ok := bigWord("日本"); ok {
		t.Fatal("letters the font lacks")
	}
	var saved Settings
	m := New("Wort", Settings{WPM: 300}, func(s Settings) { saved = s })
	m.width, m.height = 100, 20
	small := m.View()
	m.Update(key("f"))
	if !saved.Big || saved.WPM != 300 || m.View() == small || strings.Count(m.View(), "\n") < 12 {
		t.Fatalf("f: %+v\n%s", saved, m.View())
	}
	// Too narrow for the big letters: the word is shown plain.
	m.width = 20
	if !strings.Contains(m.View(), "Wort") {
		t.Fatalf("narrow:\n%s", m.View())
	}
}

func TestSeekGoesToTheStartOfTheSentence(t *testing.T) {
	m := New("One two three. Four five six. Seven.", Settings{WPM: 300}, nil)
	m.Seek(4)
	if m.chunks[m.i].word != "Four" {
		t.Fatalf("seek into a sentence: %q", m.chunks[m.i].word)
	}
	m.Seek(1000)
	if m.chunks[m.i].word != "Seven." {
		t.Fatalf("seek past the end: %q", m.chunks[m.i].word)
	}
	if i, done := m.Position(); i != 6 || !done {
		t.Fatalf("position at the end: %d %v", i, done)
	}
	m.Seek(0)
	if i, done := m.Position(); i != 6 || !done {
		t.Fatal("seek 0 is a fresh start and leaves the reader alone")
	}
	b := New("Before.\n\n```\ncode\n```\n\nAfter.", Settings{WPM: 300}, nil)
	b.Seek(1)
	if !b.paused || b.chunks[b.i].block == "" {
		t.Fatal("seek onto a block waits for space")
	}
	if i, done := New("", Settings{}, nil).Position(); i != 0 || !done {
		t.Fatal("an empty text is done")
	}
}

func TestPauseShowsTheSentence(t *testing.T) {
	m := New("First one here. The second sentence is long enough to wrap around a narrow screen width. Last.", Settings{WPM: 300}, nil)
	m.width, m.height = 40, 14
	m.Seek(5)
	m.Update(tickMsg{gen: m.gen})
	if strings.Contains(m.View(), "enough") {
		t.Fatal("the sentence shows while playing")
	}
	m.Update(key(" "))
	v := m.View()
	for _, w := range []string{"The", "second", "narrow"} {
		if !strings.Contains(v, w) {
			t.Fatalf("paused view lacks %q:\n%s", w, v)
		}
	}
	if strings.Contains(v, "First") || strings.Contains(v, "Last.") {
		t.Fatalf("other sentences shown:\n%s", v)
	}
	for _, l := range m.context(20, 2) {
		if n := len([]rune(stripANSI(l))); n > 20 {
			t.Fatalf("line of %d runes: %q", n, l)
		}
	}
	if got := len(m.context(20, 2)); got != 2 {
		t.Fatalf("%d lines, want 2", got)
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	esc := false
	for _, r := range s {
		switch {
		case r == '\x1b':
			esc = true
		case esc && (r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'):
			esc = false
		case !esc:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func TestKeysArrivingTogether(t *testing.T) {
	m := New("One two three.", Settings{WPM: 300}, nil)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("kkk")})
	if m.wpm != 375 {
		t.Fatalf("kkk: %d", m.wpm)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("kkk"), Paste: true})
	if m.wpm != 375 {
		t.Fatal("a paste is not keys")
	}
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("jq")}); cmd == nil || m.wpm != 350 {
		t.Fatal("jq: slower, then quit")
	}
}
