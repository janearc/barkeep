package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// knobs are the settings a strip turns by: the flags at the start, and
// after that whatever tmux's @ramp-speed, @ramp-span, @ramp-lightness and
// @ramp-chroma say, read every second, so a change needs no restart.
type knobs struct {
	speed, span, lightness, chroma float64
}

// drive keeps one tmux control-mode client open on a socket and, every
// frame, sets @ramp to the strip, with @bar's words on it, and asks each
// attached client to redraw its status line.
//
// Nothing is started per frame; tmux is asked about @bar and the knobs
// every second and for its clients every three.
//
// A second drive on the same socket finds the first holding the lock and
// leaves. When its own binary is rebuilt, drive becomes the new one in
// place; asked to stop, it unsets @ramp, so the status line falls back
// to whatever it showed before.
func drive(round wheel, start knobs, fps int, socket string,
	ink lamps) error {
	held, err := lock(socket)
	if err != nil || held == nil {
		return err
	}
	defer held.Close()
	in, control, err := controlled(socket)
	if err != nil {
		return err
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	tick := time.NewTicker(time.Second / time.Duration(fps))
	defer tick.Stop()
	seen, current := looking{socket: socket, knobs: start}, start
	built, phase, last := builtAt(), 0.0, time.Now()
	for {
		select {
		case <-stop:
			io.WriteString(in, "set -gu @ramp\n")
			in.Close()
			return control.Wait()
		case now := <-tick.C:
			seen.refresh(now)
			if seen.rebuilt(built) {
				in.Close()
				control.Wait()
				return become()
			}
			round, current = turned(round, current, seen.knobs)
			phase += now.Sub(last).Seconds() * current.speed
			last = now
			strip := styled(round, current.span, phase,
				seen.words, ink)
			if _, err := io.WriteString(in,
				commands(strip, seen.clients)); err != nil {
				return err
			}
		}
	}
}

// controlled is a tmux control-mode client on a socket, attached, its
// output read and let go, and what is written to it taken as commands.
func controlled(socket string) (io.WriteCloser, *exec.Cmd, error) {
	control := exec.Command("tmux", "-L", socket, "-C", "attach")
	in, err := control.StdinPipe()
	if err != nil {
		return nil, nil, err
	}
	out, err := control.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := control.Start(); err != nil {
		return nil, nil, err
	}
	go io.Copy(io.Discard, out)
	return in, control, nil
}

// turned is the wheel for new knobs: the same one while the colours are
// the same, built again when lightness or chroma moved.
func turned(round wheel, current, wanted knobs) (wheel, knobs) {
	if wanted.lightness != current.lightness ||
		wanted.chroma != current.chroma {
		lightness, chroma = wanted.lightness, wanted.chroma
		round = sampled(loop(), 8192)
	}
	return round, wanted
}

// commands are what one frame says to tmux: @ramp set to the strip,
// quoted so tmux takes it as it is, and a redraw for each client.
func commands(strip string, clients []string) string {
	said := strings.Builder{}
	fmt.Fprintf(&said, "set -g @ramp %s\n", quoted(strip))
	for _, client := range clients {
		fmt.Fprintf(&said, "refresh-client -S -t %s\n", client)
	}
	return said.String()
}

// quoted is a value in single quotes, where tmux expands nothing; a
// single quote in it closes the quotes, comes as itself in double
// quotes, and opens them again.
func quoted(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

// looking is what drive keeps an eye on, and when it last looked: the
// words in @bar and the knobs every second, the clients every three, and
// when its own binary was built.
type looking struct {
	socket        string
	words         string
	knobs         knobs
	clients       []string
	built         time.Time
	read, counted time.Time
}

// refresh looks again at whatever is due.
func (seen *looking) refresh(now time.Time) {
	if now.Sub(seen.read) >= time.Second {
		seen.words, seen.read = bar(seen.socket), now
		seen.knobs = dialled(seen.socket, seen.knobs)
		seen.built = builtAt()
	}
	if now.Sub(seen.counted) >= 3*time.Second {
		seen.clients, seen.counted = attached(seen.socket), now
	}
}

// rebuilt is whether the binary on disk is newer than the one running.
func (seen *looking) rebuilt(running time.Time) bool {
	return !seen.built.IsZero() && seen.built.After(running)
}

// builtAt is when the binary on disk was last written, or the zero time.
func builtAt() time.Time {
	path, err := os.Executable()
	if err != nil {
		return time.Time{}
	}
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// become runs the binary on disk in this process's place, with the same
// arguments: the lock and the control client go with the old image, and
// the new one takes them up again.
func become() error {
	path, err := os.Executable()
	if err != nil {
		return err
	}
	return syscall.Exec(path, os.Args, os.Environ())
}

// dialled is the knobs as tmux's @ramp- options set them, each one left
// as it was when its option is unset or not a number.
func dialled(socket string, was knobs) knobs {
	listing, err := exec.Command("tmux", "-L", socket, "show",
		"-g").Output()
	if err != nil {
		return was
	}
	now := was
	places := map[string]*float64{"@ramp-speed": &now.speed,
		"@ramp-span": &now.span, "@ramp-lightness": &now.lightness,
		"@ramp-chroma": &now.chroma}
	for line := range strings.Lines(string(listing)) {
		name, value, found := strings.Cut(strings.TrimSpace(line), " ")
		place, known := places[name]
		if !found || !known {
			continue
		}
		number, err := strconv.ParseFloat(strings.Trim(value, `"'`), 64)
		if err == nil && number >= 0 {
			*place = number
		}
	}
	return now
}

// bar is the words barkeep has put in @bar, or nothing.
func bar(socket string) string {
	said, err := exec.Command("tmux", "-L", socket, "show", "-gqv",
		"@bar").Output()
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(said), "\n")
}

// attached is the clients on a socket that draw a status line: every
// one but those in control mode, this drive's own among them.
func attached(socket string) []string {
	listing, err := exec.Command("tmux", "-L", socket, "list-clients",
		"-F", "#{client_control_mode} #{client_name}").Output()
	if err != nil {
		return nil
	}
	clients := []string{}
	for line := range strings.Lines(string(listing)) {
		mode, name, found := strings.Cut(strings.TrimSpace(line), " ")
		if found && mode == "0" {
			clients = append(clients, name)
		}
	}
	return clients
}

// lock holds a file under the home's .local/state/ramps for the socket,
// so one drive turns a server's strip; nil, with no error, when another
// drive holds it already.
func lock(socket string) (*os.File, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".local", "state", "barkeep")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filepath.Join(dir, socket+".lock"),
		os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	how := syscall.LOCK_EX | syscall.LOCK_NB
	if syscall.Flock(int(file.Fd()), how) != nil {
		file.Close()
		return nil, nil
	}
	return file, nil
}
