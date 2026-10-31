# bash completion for glance
_glance() {
  local cur=${COMP_WORDS[COMP_CWORD]} prev=${COMP_WORDS[COMP_CWORD-1]}
  case $prev in
    -w|--wpm) COMPREPLY=($(compgen -W "200 250 300 350 400 450 500 600 700 800" -- "$cur")); return ;;
  esac
  if [[ $cur == -* ]]; then
    COMPREPLY=($(compgen -W "-w --wpm -b --big --fresh -p --popup -v --version -h --help" -- "$cur"))
    return
  fi
  compopt -o default 2>/dev/null
  COMPREPLY=()
}
complete -F _glance glance
