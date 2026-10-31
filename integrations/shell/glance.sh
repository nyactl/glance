# glance helpers for bash and zsh. Source it from .bashrc or .zshrc:
#   . /path/to/glance/integrations/shell/glance.sh

# glance-clip reads the clipboard.
glance-clip() {
  if command -v pbpaste >/dev/null 2>&1; then pbpaste
  elif command -v wl-paste >/dev/null 2>&1; then wl-paste --no-newline
  elif command -v xclip >/dev/null 2>&1; then xclip -selection clipboard -o
  elif command -v xsel >/dev/null 2>&1; then xsel --clipboard --output
  else echo "glance-clip: no clipboard tool (pbpaste, wl-paste, xclip, xsel)" >&2; return 1
  fi | glance "$@"
}

# glance-url reads a web page as text, with whichever of these is installed.
glance-url() {
  [ -n "$1" ] || { echo "usage: glance-url URL [flags]" >&2; return 2; }
  url=$1; shift
  if command -v w3m >/dev/null 2>&1; then w3m -dump "$url"
  elif command -v lynx >/dev/null 2>&1; then lynx -dump -nolist "$url"
  elif command -v pandoc >/dev/null 2>&1; then curl -fsSL "$url" | pandoc -f html -t plain
  else echo "glance-url: needs w3m, lynx or pandoc" >&2; return 1
  fi | glance "$@"
}

# glance-pdf reads a PDF, with pdftotext from poppler.
glance-pdf() {
  [ -n "$1" ] || { echo "usage: glance-pdf FILE [flags]" >&2; return 2; }
  file=$1; shift
  pdftotext -layout "$file" - | glance "$@"
}

