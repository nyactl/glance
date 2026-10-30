# glance

A terminal speed reader: it shows a text one word at a time at a fixed spot, with the letter the
eye rests on marked, so it reads faster than it scrolls. Markdown is read as text; code blocks and
tables are shown whole.

```sh
go install github.com/nyactl/glance@latest

glance notes.md          # a file
pbpaste | glance         # what you copied
some-command | glance    # anything piped in
```

| Key           | Does                                   |
|---------------|----------------------------------------|
| space, enter  | pause and go on                        |
| h / l         | a sentence back, a sentence forward    |
| j / k         | slower, faster (25 words a minute)     |
| f             | big letters                            |
| q, esc        | quit                                   |

The speed and the letter size are kept for next time in `$XDG_STATE_HOME/glance/settings.json`
(default `~/.local/state`).
