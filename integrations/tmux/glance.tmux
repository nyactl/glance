# glance in tmux. Source it from tmux.conf:
#   source-file /path/to/glance/integrations/tmux/glance.tmux
# Each binding reads in a popup over the current pane, so the pane stays as it is.

# prefix + R: read what the current pane shows, from the top of its history.
bind-key R run-shell "tmux capture-pane -p -J -S - | glance --popup"

# prefix + B: read the last copied text (the top paste buffer).
bind-key B run-shell "tmux show-buffer | glance --popup"

# R in copy mode: read the selection.
bind-key -T copy-mode-vi R send-keys -X copy-pipe-and-cancel "glance --popup"
bind-key -T copy-mode R send-keys -X copy-pipe-and-cancel "glance --popup"
