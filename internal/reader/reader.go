// Package reader shows a text one word at a time at a fixed spot, so it reads faster than it
// scrolls. Code blocks and tables do not read word by word and are shown whole.
package reader

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	figure "github.com/common-nighthawk/go-figure"
)

const (
	// DefaultWPM is the speed until the user picks another.
	DefaultWPM = 350
	minWPM     = 100
	maxWPM     = 1000
	stepWPM    = 25
)

// chunk is a word, or a block shown whole.
type chunk struct {
	word  string
	block string
	// start marks the first word of a sentence, where back and forward go.
	start bool
	// weight stretches the time a word stays: longer after a sentence or a long word.
	weight float64
}

var (
	listMark = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)])\s+`)
	heading  = regexp.MustCompile(`^\s*#{1,6}\s+`)
	// inline markup that is not read: emphasis, code ticks, link targets.
	markup = strings.NewReplacer("**", "", "__", "", "`", "")
	link   = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
)

// split cuts a markdown text into words and whole blocks.
func split(text string) []chunk {
	var out []chunk
	start := true
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "```"):
			j := i + 1
			for j < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[j]), "```") {
				j++
			}
			out = append(out, chunk{block: strings.Join(lines[i+1:min(j, len(lines))], "\n"), start: true})
			i, start = j, true
			continue
		case strings.HasPrefix(trimmed, "|"):
			j := i
			for j < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[j]), "|") {
				j++
			}
			out = append(out, chunk{block: strings.Join(lines[i:j], "\n"), start: true})
			i, start = j-1, true
			continue
		case trimmed == "":
			if n := len(out); n > 0 && out[n-1].block == "" {
				out[n-1].weight = max(out[n-1].weight, 2.5)
			}
			start = true
			continue
		}
		if heading.MatchString(line) || listMark.MatchString(line) {
			start = true
		}
		line = heading.ReplaceAllString(listMark.ReplaceAllString(line, ""), "")
		line = markup.Replace(link.ReplaceAllString(line, "$1"))
		for _, w := range strings.Fields(line) {
			c := chunk{word: w, start: start, weight: 1}
			switch w[len(w)-1] {
			case '.', '!', '?':
				c.weight = 2.2
			case ',', ';', ':':
				c.weight = 1.5
			}
			if utf8.RuneCountInString(w) > 8 {
				c.weight += 0.3
			}
			out = append(out, c)
			start = strings.ContainsAny(w[len(w)-1:], ".!?")
		}
		// A line break inside a paragraph rarely ends a sentence; a list item or heading does.
		if n := len(out); n > 0 && (i+1 < len(lines) && (listMark.MatchString(lines[i+1]) || heading.MatchString(lines[i+1]))) {
			out[n-1].weight = max(out[n-1].weight, 2)
		}
	}
	return out
}

// focus is the letter the eye rests on, a little left of the middle of a word.
func focus(word string) int {
	switch n := utf8.RuneCountInString(word); {
	case n <= 1:
		return 0
	case n <= 5:
		return 1
	case n <= 9:
		return 2
	case n <= 13:
		return 3
	default:
		return 4
	}
}

type tickMsg struct{ gen int }

// Settings are what the reader remembers between texts.
type Settings struct {
	WPM int  `json:"wpm"`
	Big bool `json:"big"`
}

// Model plays a text.
type Model struct {
	chunks        []chunk
	i             int
	wpm           int
	big           bool
	paused        bool
	gen           int
	width, height int
	// save keeps changed settings for the next time.
	save func(Settings)
}

// New makes a reader for text with the settings of last time; save keeps them when they change.
func New(text string, s Settings, save func(Settings)) *Model {
	wpm := s.WPM
	if wpm < minWPM || wpm > maxWPM {
		wpm = DefaultWPM
	}
	m := &Model{chunks: split(text), wpm: wpm, big: s.Big, save: save, width: 80, height: 12}
	m.paused = len(m.chunks) == 0 || m.chunks[0].block != ""
	return m
}

func (m *Model) Init() tea.Cmd {
	if m.paused {
		return nil
	}
	// A moment to find the spot before the first word moves on.
	return m.tick(2)
}

func (m *Model) tick(weight float64) tea.Cmd {
	gen := m.gen
	d := time.Duration(weight * float64(time.Minute) / float64(m.wpm))
	return tea.Tick(d, func(time.Time) tea.Msg { return tickMsg{gen} })
}

// restart starts the timer again from the current chunk, dropping any running one.
func (m *Model) restart() tea.Cmd {
	m.gen++
	if m.paused || m.i >= len(m.chunks) {
		return nil
	}
	if m.chunks[m.i].block != "" {
		m.paused = true
		return nil
	}
	return m.tick(m.chunks[m.i].weight)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tickMsg:
		if msg.gen != m.gen || m.paused {
			return m, nil
		}
		m.i++
		if m.i >= len(m.chunks) {
			m.i, m.paused = len(m.chunks)-1, true
			return m, nil
		}
		return m, m.restart()
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case " ", "enter":
			if m.paused && m.i < len(m.chunks) && m.chunks[m.i].block != "" {
				// A block was read: go on with what follows.
				m.i = min(m.i+1, len(m.chunks)-1)
			} else if m.paused && m.i == len(m.chunks)-1 {
				m.i = 0
			}
			m.paused = !m.paused
			return m, m.restart()
		case "h", "left":
			m.i = m.sentence(-1)
			return m, m.restart()
		case "l", "right":
			m.i = m.sentence(1)
			return m, m.restart()
		case "k", "up", "+":
			m.setWPM(m.wpm + stepWPM)
		case "j", "down", "-":
			m.setWPM(m.wpm - stepWPM)
		case "f":
			m.big = !m.big
			m.keep()
		}
	}
	return m, nil
}

func (m *Model) setWPM(wpm int) {
	m.wpm = min(max(wpm, minWPM), maxWPM)
	m.keep()
}

func (m *Model) keep() {
	if m.save != nil {
		m.save(Settings{WPM: m.wpm, Big: m.big})
	}
}

// bigFont is a figlet font with lower case letters that still reads at a glance.
const bigFont = "colossal"

// asciiOf spells a word in the letters the big font has; it has no others.
var asciiOf = strings.NewReplacer("ä", "ae", "ö", "oe", "ü", "ue", "Ä", "Ae", "Ö", "Oe", "Ü", "Ue", "ß", "ss",
	"’", "'", "‘", "'", "“", "\"", "”", "\"", "–", "-", "—", "-", "…", "...")

// glyphs holds the letters of the big font drawn so far.
var glyphs = map[rune][]string{}

// bigWord draws a word in the big font, letter by letter so the focus letter can be found: the
// rows, and the columns where the focus letter starts and ends. ok is false for a word the font
// cannot draw.
func bigWord(word string) (rows []string, from, to int, ok bool) {
	word = asciiOf.Replace(word)
	f := focus(word)
	var drawn [][]string
	height := 0
	for _, r := range word {
		if r < 33 || r > 126 {
			return nil, 0, 0, false
		}
		g, ok := glyphs[r]
		if !ok {
			// Reading the font is slow enough to show at a few hundred words a minute.
			g = figure.NewFigure(string(r), bigFont, true).Slicify()
			glyphs[r] = g
		}
		height = max(height, len(g))
		drawn = append(drawn, g)
	}
	rows = make([]string, height)
	for i, g := range drawn {
		w := 0
		for _, l := range g {
			w = max(w, len(l))
		}
		if i == f {
			from = len(rows[0])
		}
		for j := range rows {
			l := ""
			if j < len(g) {
				l = g[j]
			}
			rows[j] += l + strings.Repeat(" ", w-len(l)+1)
		}
		if i == f {
			to = len(rows[0]) - 1
		}
	}
	return rows, from, to, true
}

// sentence is the start of the sentence dir sentences away: back first goes to the start of the
// current one, unless it is there already.
func (m *Model) sentence(dir int) int {
	if len(m.chunks) == 0 {
		return 0
	}
	i := m.i
	if dir < 0 {
		if i > 0 && !m.chunks[i].start {
			for i > 0 && !m.chunks[i].start {
				i--
			}
			return i
		}
		i--
		for i > 0 && !m.chunks[i].start {
			i--
		}
		return max(i, 0)
	}
	i++
	for i < len(m.chunks) && !m.chunks[i].start {
		i++
	}
	return min(i, len(m.chunks)-1)
}

var (
	focusStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1"))
	dimStyle   = lipgloss.NewStyle().Faint(true)
)

func (m *Model) View() string {
	if len(m.chunks) == 0 {
		return "nothing to read; q quits\n"
	}
	c := m.chunks[m.i]
	var b strings.Builder
	if c.block != "" {
		b.WriteString(dimStyle.Render("shown whole; space goes on") + "\n\n")
		lines := strings.Split(c.block, "\n")
		for _, l := range lines[:min(len(lines), max(m.height-5, 1))] {
			if r := []rune(l); len(r) > m.width {
				l = string(r[:m.width])
			}
			b.WriteString(l + "\n")
		}
	} else if rows, from, to, ok := bigWord(c.word); m.big && ok && len(rows[0]) < m.width && len(rows)+4 <= m.height {
		center := m.width / 3
		pad := max(center-(from+to)/2, 0)
		if pad+len(rows[0]) > m.width {
			pad = max(m.width-len(rows[0]), 0)
		}
		top := max((m.height-len(rows)-3)/2, 0)
		b.WriteString(strings.Repeat("\n", top))
		for _, r := range rows {
			r = strings.TrimRight(r+strings.Repeat(" ", max(to+1-len(r), 0)), " ")
			line := strings.Repeat(" ", pad)
			switch {
			case len(r) <= from:
				line += r
			case len(r) <= to:
				line += r[:from] + focusStyle.Render(r[from:])
			default:
				line += r[:from] + focusStyle.Render(r[from:to]) + r[to:]
			}
			b.WriteString(line + "\n")
		}
		b.WriteString(strings.Repeat("\n", max(m.height-top-len(rows)-2, 0)))
	} else {
		center := m.width / 3
		r := []rune(c.word)
		f := focus(c.word)
		pad := max(center-f, 0)
		top := max((m.height-5)/2, 0)
		b.WriteString(strings.Repeat("\n", top))
		b.WriteString(strings.Repeat(" ", center) + dimStyle.Render("|") + "\n")
		b.WriteString(strings.Repeat(" ", pad) + string(r[:f]) + focusStyle.Render(string(r[f])) + string(r[f+1:]) + "\n")
		b.WriteString(strings.Repeat(" ", center) + dimStyle.Render("|") + "\n")
		b.WriteString(strings.Repeat("\n", max(m.height-top-5, 0)))
	}
	state := ""
	switch {
	case m.paused && m.i == len(m.chunks)-1 && c.block == "":
		state = "the end; space from the start, "
	case m.paused:
		state = "paused, "
	}
	b.WriteString(dimStyle.Render(fmt.Sprintf("  %d%%  %d wpm  %sspace pause  h/l sentence  j/k speed  f size  q quit",
		100*(m.i+1)/len(m.chunks), m.wpm, state)))
	return b.String()
}
