# glance

[![Release](https://img.shields.io/github/v/release/nyactl/glance)](https://github.com/nyactl/glance/releases/latest)
[![CI](https://github.com/nyactl/glance/actions/workflows/ci.yml/badge.svg)](https://github.com/nyactl/glance/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/github/license/nyactl/glance)](LICENSE)

A terminal speed reader. glance shows a text one word at a time at a fixed spot, with the letter
the eye rests on marked, so it reads faster than it scrolls. Markdown is read as text; code
blocks and tables are shown whole.

```sh
glance notes.md          # a file
pbpaste | glance         # what you copied
some-command | glance    # anything piped in
```

## Why this one?

- **Moves by sentence.** `h` and `l` jump a sentence back and forward, so missing a word costs
  one sentence, not a restart. A pause shows the whole sentence around the current word.
- **Vim keys**, and the keys still work when the text comes from a pipe.
- **Goes on where you left off.** Each text is remembered by its content, from a file or a
  pipe; `--fresh` starts over.
- **Reads Markdown the way it is meant**: headings, list marks, emphasis and link targets are
  left out, code blocks and tables are shown whole and wait for you.
- **Natural pacing**: words stay longer after a comma, at the end of a sentence or a paragraph,
  and when they are long.
- **Big letters** (`f`) for reading from across the room.
- **Fits in**: a tmux popup mode, a Vim and Neovim plugin, shell helpers, completions and a man
  page come with it.
- A single Go binary for macOS and Linux; releases are signed.

## Install

**Homebrew** (macOS and Linux), with the man page, completions and integrations:

```sh
brew install nyactl/tap/glance
```

**Install script**, to `~/.local/bin` (or `GLANCE_DIR`), checked against the release checksums:

```sh
curl -fsSL https://raw.githubusercontent.com/nyactl/glance/main/install.sh | sh
```

**Debian, Ubuntu, Fedora, RHEL**: get the `.deb` or `.rpm` from the
[releases](https://github.com/nyactl/glance/releases/latest), then

```sh
sudo dpkg -i glance_<version>_linux_amd64.deb
sudo rpm -i glance_<version>_linux_amd64.rpm
```

**Go**:

```sh
go install github.com/nyactl/glance@latest
```

## Use

```
glance [flags] FILE
COMMAND | glance [flags]    or glance - to name stdin

  -w, --wpm N    start at N words a minute (100-1000; default: the speed of last time)
  -b, --big      big letters
      --fresh    start at the beginning, not where this text was left
  -p, --popup    inside tmux, read in a popup over the current pane
  -v, --version  print the version
  -h, --help     this help
```

| Key                  | Does                                                |
|----------------------|-----------------------------------------------------|
| space, enter         | pause and go on; at the end, start over             |
| h / l, left / right  | a sentence back, a sentence forward                 |
| j / k, down / up     | slower, faster, by 25 words a minute                |
| f                    | big letters                                         |
| q, esc               | quit                                                |

The speed and the letter size are kept for next time, and where each of the last 200 texts was
left, in `$XDG_STATE_HOME/glance` (default `~/.local/state/glance`). `NO_COLOR` turns the colour
of the focus letter off.

## Recipes

```sh
glance-clip                                  # the clipboard, on macOS, Wayland or X (shell helpers)
pdftotext paper.pdf - | glance -w 450        # a PDF
w3m -dump https://example.com | glance       # a web page
man tar | col -b | glance                    # a man page
git log -1 --format=%B | glance              # the last commit message
gh pr view 123 | glance                      # a pull request
```

## Integrations

Everything here is in [`integrations/`](integrations); Homebrew puts it in
`$(brew --prefix)/share/glance/integrations`, the Linux packages in `/usr/share/glance/integrations`.

### tmux

`glance --popup` reads in a popup over the current pane. It needs no terminal of its own, so it
works from tmux bindings and from editors. [`glance.tmux`](integrations/tmux/glance.tmux) binds:

| Binding           | Reads                                       |
|-------------------|---------------------------------------------|
| prefix + R        | what the current pane shows, with history   |
| prefix + B        | the last copied text (top paste buffer)     |
| R in copy mode    | the selection                               |

```tmux
source-file /path/to/glance/integrations/tmux/glance.tmux
```

### Vim and Neovim

`:Glance` reads the buffer, `:'<,'>Glance` the selected lines, and flags pass through
(`:Glance -w 500`). Inside tmux it uses a popup; elsewhere a terminal window, floating in Neovim.

With [lazy.nvim](https://github.com/folke/lazy.nvim):

```lua
{ "nyactl/glance", config = function(p) vim.opt.rtp:append(p.dir .. "/integrations/vim") end }
```

With [vim-plug](https://github.com/junegunn/vim-plug):

```vim
Plug 'nyactl/glance', { 'rtp': 'integrations/vim' }
```

Or copy [`glance.vim`](integrations/vim/plugin/glance.vim) to `~/.vim/plugin/`. A mapping, for
example: `vnoremap <leader>g :Glance<CR>`.

### Shell

[`glance.sh`](integrations/shell/glance.sh) adds, for bash and zsh, `glance-clip` (the clipboard),
`glance-url URL` (a web page, with w3m, lynx or pandoc) and `glance-pdf FILE` (with pdftotext).

```sh
. /path/to/glance/integrations/shell/glance.sh
```

### Completions and man page

Homebrew and the Linux packages install them. By hand: [`completions/`](completions) has bash,
zsh and fish; [`man/glance.1`](man/glance.1) is the man page.

## Verify a release

Every release's `checksums.txt` is signed with [cosign](https://github.com/sigstore/cosign),
keyless, through the release workflow:

```sh
VERSION=v0.2.0
curl -LO https://github.com/nyactl/glance/releases/download/$VERSION/checksums.txt
curl -LO https://github.com/nyactl/glance/releases/download/$VERSION/checksums.txt.sigstore.json
cosign verify-blob --bundle checksums.txt.sigstore.json \
  --certificate-identity "https://github.com/nyactl/glance/.github/workflows/release.yml@refs/tags/$VERSION" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com checksums.txt
sha256sum --check --ignore-missing checksums.txt
```

## Develop

```sh
make test     # unit and end-to-end tests; the end-to-end ones run the binary in a terminal
make cover    # the same with coverage, which must stay at 80% or more
```

`go test -short ./...` skips the end-to-end tests. Releases are described in
[RELEASING.md](RELEASING.md).

## License

[MIT](LICENSE)
