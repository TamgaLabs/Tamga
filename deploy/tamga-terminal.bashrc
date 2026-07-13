# Tamga's terminal exec sessions always run interactive Bash. Keep this file
# deliberately small: it configures the sandbox shell, not the browser
# terminal protocol.
case $- in
  *i*) ;;
  *) return ;;
esac

# Alpine's bash-completion package wires command, path, and Git completion
# through /etc/bash/bashrc. Source it explicitly because Docker exec starts a
# non-login interactive shell, which does not read /etc/profile.
if [ -r /etc/bash/bashrc ]; then
  . /etc/bash/bashrc
fi

# A sandbox belongs to exactly one project. Its terminal sessions share this
# file only while that sandbox container is alive; AgentService removes the
# container when its final session ends. Nothing is mirrored to the browser.
export HISTFILE=/tmp/.tamga-bash-history
export HISTSIZE=10000
export HISTFILESIZE=20000
export HISTCONTROL=ignoredups:erasedups
shopt -s histappend

__tamga_history_sync() {
  history -a
  history -n
}

# Append this shell's completed command and read commands appended by sibling
# terminal tabs before every prompt. Preserve a pre-existing prompt hook in
# case an installed tool configures one later.
PROMPT_COMMAND="__tamga_history_sync${PROMPT_COMMAND:+; $PROMPT_COMMAND}"

# Make an already-running sibling session's history available immediately to
# a newly opened tab, rather than waiting for its first command to finish.
if [ -f "$HISTFILE" ]; then
  history -n
fi
