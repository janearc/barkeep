#!/bin/sh
# measure.sh: what the strip costs in a tmux status line at about ten
# frames a second, done the expensive way: every frame starts ramps and
# two tmux clients afresh. Runs on its own socket, ramps-test, with one
# client attached through script's pty so the status line is drawn.
# Prints cpu seconds used over the run by the driver (ramps and the tmux
# commands it starts), the tmux server, and the attached client.
set -u
sock=ramps-test
here=$(cd "$(dirname "$0")" && pwd)
seconds=${1:-30}

tmux -L $sock -f /dev/null new-session -d -s t -x 177 -y 40
tmux -L $sock set -g status 2
tmux -L $sock set -g 'status-format[1]' '#{@ramp}'
sleep 600 | TERM=xterm-256color script -q /dev/null \
	tmux -L $sock attach -t t >/dev/null 2>&1 &
sleep 2
client=$(tmux -L $sock list-clients -F '#{client_name}' | head -1)
server=$(tmux -L $sock display -p '#{pid}')
clientpid=$(tmux -L $sock list-clients -F '#{client_pid}' | head -1)
echo "client $client, server pid $server, client pid $clientpid"

cpu() { ps -o time= -p "$1" | awk -F'[:.]' '{print $1*60+$2+$3/100}'; }
server0=$(cpu "$server"); client0=$(cpu "$clientpid")

/usr/bin/time -p sh -c '
	end=$(( $(date +%s) + '"$seconds"' ))
	frames=0
	while [ $(date +%s) -lt $end ]; do
		tmux -L '$sock' set -g @ramp "$('"$here"'/ramps -tmux)"
		tmux -L '$sock' refresh-client -S -t "'"$client"'"
		frames=$((frames + 1))
		sleep 0.1
	done
	echo "frames $frames in '"$seconds"' s"
' 2>&1 | grep -E 'frames|user|sys'

echo "server cpu s: $(echo "$(cpu "$server") - $server0" | bc)"
echo "client cpu s: $(echo "$(cpu "$clientpid") - $client0" | bc)"
tmux -L $sock kill-server
pkill -f "script -q /dev/null tmux -L $sock" 2>/dev/null
pkill -f "sleep 600" 2>/dev/null
exit 0
