" glance for Vim and Neovim: :Glance reads the buffer, :'<,'>Glance the selected lines.
" Inside tmux it reads in a popup, elsewhere in a terminal window (a floating one in Neovim).
" Add integrations/vim to 'runtimepath', or copy this file to ~/.vim/plugin.

if exists('g:loaded_glance')
  finish
endif
let g:loaded_glance = 1

let g:glance_command = get(g:, 'glance_command', 'glance')

function! s:glance(line1, line2, args) abort
  let l:file = tempname()
  call writefile(getline(a:line1, a:line2), l:file)
  let l:cmd = [g:glance_command] + split(a:args) + [l:file]
  if !empty($TMUX)
    call system(join(map([g:glance_command, '--popup'] + split(a:args) + [l:file], 'shellescape(v:val)')))
    call delete(l:file)
    if v:shell_error
      echohl ErrorMsg | echom 'glance failed' | echohl None
    endif
    return
  endif
  if has('nvim')
    let l:w = float2nr(&columns * 0.8)
    let l:h = float2nr(&lines * 0.6)
    let l:buf = nvim_create_buf(v:false, v:true)
    call nvim_open_win(l:buf, v:true, {'relative': 'editor', 'width': l:w, 'height': l:h,
          \ 'row': (&lines - l:h) / 2, 'col': (&columns - l:w) / 2, 'style': 'minimal', 'border': 'rounded'})
    call termopen(l:cmd, {'on_exit': {-> [delete(l:file), execute('silent! bdelete! ' . l:buf)]}})
    startinsert
  else
    call term_start(l:cmd, {'term_finish': 'close', 'exit_cb': {-> delete(l:file)}})
  endif
endfunction

command! -range=% -nargs=* Glance call s:glance(<line1>, <line2>, <q-args>)
