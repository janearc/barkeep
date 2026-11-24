#!/bin/sh
# measure-drive.sh: what the strip costs at ten frames a second done the
# cheap way, one long-lived ramps -drive holding one tmux control-mode
# client. Same test server as measure.sh, on its own socket, ramps-test.
# Prints cpu seconds over the run for ramps, its control client, the tmux
# server and the attached client, and whether @ramp moved.
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

cpu() { ps -o time= -p "$1" | awk -F'[:.]' '{print $1*60+$2+$3/100}'; }
server0=$(cpu "$server"); client0=$(cpu "$clientpid")

"$here/ramps" -drive $sock -fps 10 &
driver=$!
sleep 2
control=$(pgrep -P $driver tmux)
first=$(tmux -L $sock show -gv @ramp | cksum)
sleep 1
second=$(tmux -L $sock show -gv @ramp | cksum)
[ "$first" != "$second" ] && echo "@ramp moves" || echo "@ramp still"
sleep $((seconds - 3))

echo "ramps cpu s:   $(cpu "$driver")"
echo "control cpu s: $(cpu "$control")"
echo "server cpu s:  $(echo "$(cpu "$server") - $server0" | bc)"
echo "client cpu s:  $(echo "$(cpu "$clientpid") - $client0" | bc)"
kill $driver
tmux -L $sock kill-server
pkill -f "script -q /dev/null tmux -L $sock" 2>/dev/null
pkill -f "sleep 600" 2>/dev/null
exit 0
