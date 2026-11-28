package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
)

// A strip is one narrow pane the column is drawn in: where it is written,
// how big it was when last drawn, and each row as last written, so a row
// that has not changed is not written again.
type strip struct {
	out           int
	name          string
	columns, rows int
	drawn         []string
	gone          bool
}

// A cell is one character of a strip: its glyph, the ink it is drawn in
// and the ground under it. A half block's ink is its row's upper colour
// and its ground the lower. A cell of the readings says so, so an effect
// can leave it readable.
//
// A glyph two cells wide, an emoji, is a wide cell followed by an under
// cell, which the glyph covers and which writes nothing of its own.
type cell struct {
	glyph       rune
	ink, ground lamps
	words       bool
	wide, under bool
}

// halfBlock is the glyph the ramp is drawn with: its ink over its ground.
const halfBlock = '▀'

// opened is a pane's terminal, opened for writing only and never to
// wait: a write the terminal has no room for comes back at once.
func opened(path string) (strip, error) {
	out, err := syscall.Open(path,
		syscall.O_WRONLY|syscall.O_NONBLOCK|syscall.O_NOCTTY, 0)
	if err != nil {
		return strip{}, fmt.Errorf("%s: %w", path, err)
	}
	return strip{out: out, name: path}, nil
}

// readEvery is how often the machine's readings are taken.
const readEvery = 2 * time.Second

// firstLine is the strip's row the readings begin on, counted from
// nought: the top.
const firstLine = 0

// A moment is what the ramp is at an instant: the loop, how much of it
// a strip spans, and how far it has turned.
type moment struct {
	round        wheel
	span, turned float64
}

// written is the words on the strip: the readings' lines, the ink they
// are written in before libreadme moves it to read, and when they were
// last taken.
type written struct {
	lines []line
	ink   lamps
	read  time.Time
	game  *gameBoard
	age   time.Duration
}

// column draws the loop down every strip, the same picture in each,
// turning slowly, until it is told to stop: its own pane, and the pane
// of every terminal named. The optional Game poll runs off the draw loop.
//
// It opens with the rain, plays it again on SIGUSR1, and plays the lava
// lamp on SIGUSR2, and the hounds over a lava lamp on SIGINFO. Started
// at a terminal it takes keys as well (keys.go).
//
// it never waits long on a terminal. A row a terminal has no room for
// is drawn the next time. A strip whose pane has gone is left out from
// then on, and with none left it ends.
func column(round wheel, set knobs, fps int, ink lamps,
	others []string, game string) error {
	if err := syscall.SetNonblock(syscall.Stdout, true); err != nil {
		return err
	}
	strips := []*strip{{out: syscall.Stdout, name: "this pane"}}
	for _, path := range others {
		made, err := opened(path)
		if err != nil {
			return err
		}
		strips = append(strips, &made)
	}
	stop, again := make(chan os.Signal, 1), make(chan os.Signal, 1)
	lamp := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	signal.Notify(again, syscall.SIGUSR1)
	signal.Notify(lamp, syscall.SIGUSR2)
	dogs := make(chan os.Signal, 1)
	houndsOn(dogs)
	keys, restore := typed()
	defer restore()
	began := time.Now()
	shown, on := written{ink: ink}, stage{ink: ink}
	var boards <-chan gameBoard
	board := gameBoard{}
	if game != "" && banner == nil {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		boards = watchGame(ctx, game)
		shown.game = &board
	}
	on.put(rain(uint64(began.UnixNano())), began)
	if houndsFirst {
		on.light(on.burning.after(began, true, len(inPlay)))
	}
	if showing {
		on.put(show(round, set, ink, uint64(began.UnixNano())), began)
	}
	for {
		select {
		case <-stop:
			leave(strips)
			return nil
		case key := <-keys:
			asked := pressed(key)
			if asked == asksToLeave {
				leave(strips)
				return nil
			}
			on.press(asked, time.Now())
		case board = <-boards:
		case <-again:
			now := time.Now()
			on.put(rain(uint64(now.UnixNano())), now)
		case <-lamp:
			on.light(on.burning.after(time.Now(), false,
				len(inPlay)))
		case <-dogs:
			on.light(on.burning.after(time.Now(), true,
				len(inPlay)))
		case now := <-time.After(on.playing.pause(fps)):
			// the show, when nothing is playing, begins again
			if showing && !on.playing.live {
				on.put(show(round, set, ink,
					uint64(now.UnixNano())), now)
			}
			// a banner has no readings beside it; none are taken
			if banner == nil && now.Sub(shown.read) >= readEvery {
				shown.lines, shown.read = status(now), now
			}
			shown.age = now.Sub(began)
			at := moment{round, set.span, shown.age.Seconds() * set.speed}
			over := on.playing.at(now)
			if left := drawAll(strips, at, shown, over); left == 0 {
				return errors.New("every strip's pane has gone")
			}
		}
	}
}

// leave clears every strip and gives each its cursor back.
func leave(strips []*strip) {
	for _, each := range strips {
		each.say("\x1b[0m\x1b[2J\x1b[H\x1b[?25h")
	}
}

// drawAll draws one moment in every strip that is still there, and says
// how many are. The picture is worked out once for each size of strip
// there is, not once a strip: seven strips of one size are one picture.
func drawAll(strips []*strip, at moment, shown written, over overlay) int {
	left, pictures := 0, map[[2]int][]string{}
	for _, each := range strips {
		if each.gone || !each.sized() {
			continue
		}
		size := [2]int{each.columns, each.rows}
		rows, worked := pictures[size]
		if !worked {
			rows = pictured(size, at, shown, over)
			pictures[size] = rows
		}
		each.write(rows)
		if !each.gone {
			left++
		}
	}
	return left
}

// sized reads the strip's size and says whether it has one. A strip
// that has changed size, or was never cleared, is cleared to be drawn
// whole; one whose terminal will not say its size has gone.
func (made *strip) sized() bool {
	columns, rows, err := term.GetSize(made.out)
	if err != nil || columns <= 0 || rows <= 0 {
		made.gone = true
		return false
	}
	if columns != made.columns || rows != made.rows || made.drawn == nil {
		if !made.say("\x1b[0m\x1b[?25l\x1b[?7l\x1b[2J") {
			made.drawn = nil
			return !made.gone
		}
		made.columns, made.rows = columns, rows
		made.drawn = make([]string, rows)
	}
	return true
}

// write writes the rows of a picture that have changed since the strip
// was last written, a row at a time, each its own small write: a
// terminal takes about a thousand bytes at once, and a strip's forty
// rows are more.
//
// The first row it has no room for ends this moment's writing, and that
// row and the ones under it are written the next.
func (made *strip) write(rows []string) {
	if made.drawn == nil {
		return
	}
	for row, line := range rows {
		if line == made.drawn[row] {
			continue
		}
		if !made.say(fmt.Sprintf("\x1b[%d;1H%s\x1b[0m", row+1, line)) {
			made.drawn[row] = ""
			return
		}
		made.drawn[row] = line
	}
}

// pictured is a strip of a size at a moment, as the rows a terminal
// takes: the ramp, the words over it, and whatever effect is playing
// over both.
func pictured(size [2]int, at moment, shown written, over overlay) []string {
	if at.span <= 0 {
		at.span = at.round.fitted(size[1])
	}
	cells := cellsAt(size, at, shown)
	over.change(cells)
	rows := make([]string, len(cells))
	for row, each := range cells {
		rows[row] = encoded(each)
	}
	return rows
}

// cellsAt is every cell of a strip so many columns and rows at a
// moment: the loop running down it, two colours a row by half blocks,
// the strip's width across, and the words' lines written over it from
// firstLine down.
//
// With a banner there is none of that: the banner alone (banner.go).
func cellsAt(size [2]int, now moment, shown written) [][]cell {
	if banner != nil {
		return bannerCells(size, banner, shown.ink)
	}
	return loopAt(size, now, shown)
}

// loopAt is every cell of a strip that shows the loop: the loop running
// down it and the words' lines over it.
func loopAt(size [2]int, now moment, shown written) [][]cell {
	cells := make([][]cell, size[1])
	samples := float64(size[1] * 2)
	lines := shown.lines
	if shown.game != nil {
		game := shown.game.gameLines(size[0], size[1]-len(lines), shown.age)
		lines = append(lines, game...)
	}
	for row := range cells {
		upper := at(now.round.colours,
			float64(row*2)/samples/now.span+now.turned)
		lower := at(now.round.colours,
			float64(row*2+1)/samples/now.span+now.turned)
		said := line{}
		if at := row - firstLine; at >= 0 && at < len(lines) {
			said = lines[at]
		}
		cells[row] = rowOf(size[0], upper, lower, said, shown.ink)
	}
	return cells
}

// rowOf is one row's cells: half blocks, and over them its words, in
// the ink moved by libreadme until it reads on the row's colour. A rule
// is dashes the whole width. A letter's ground is one colour, halfway
// between the row's two, since a letter takes the place of both halves.
func rowOf(columns int, upper, lower lamps, said line, ink lamps) []cell {
	cells := make([]cell, columns)
	for column := range cells {
		cells[column] = cell{glyph: halfBlock, ink: upper,
			ground: lower}
	}
	under := mixed(upper, lower)
	reads := inkOn(ink, under)
	for column, letter := range lettersOf(columns, said) {
		if letter == ' ' {
			continue
		}
		cells[column] = cell{glyph: letter, ink: reads, ground: under,
			words: true}
	}
	return cells
}

// gap is the cells kept clear between a name and its reading.
const gap = 1

// lettersOf is a line's letters, one to a column of the row and a space
// where the row shows through: the name from the left and the reading
// set to the right. On a row too narrow for both, the reading is kept
// whole and the name is cut short, since the reading is what changes.
func lettersOf(columns int, said line) []rune {
	if said.rule {
		return []rune(strings.Repeat("-", columns))
	}
	reading := []rune(said.reading)
	if len(reading) > columns {
		reading = reading[:columns]
	}
	name := []rune(said.name)
	if room := max(0, columns-len(reading)-gap); len(name) > room {
		name = name[:room]
	}
	letters := []rune(strings.Repeat(" ", columns))
	copy(letters, name)
	copy(letters[columns-len(reading):], reading)
	return letters
}

// encoded is a row of cells as a terminal takes it: each run of cells
// in one ink on one ground behind a single pair of colour codes.
func encoded(cells []cell) string {
	row := strings.Builder{}
	for index, each := range cells {
		if index == 0 || each.ink != cells[index-1].ink ||
			each.ground != cells[index-1].ground {
			fmt.Fprintf(&row,
				"\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm",
				each.ink[0], each.ink[1], each.ink[2],
				each.ground[0], each.ground[1], each.ground[2])
		}
		if !each.under {
			row.WriteRune(each.glyph)
		}
	}
	return row.String()
}

// patience is how long say will keep offering words to a terminal that
// has no room for them, and breath how long it waits between offers.
//
// A terminal takes about a thousand bytes and then wants a moment for
// tmux to read them; a tmux that has stopped reading costs a strip this
// long a row and no more.
const (
	patience = 10 * time.Millisecond
	breath   = 200 * time.Microsecond
)

// say writes words to the strip's terminal and says whether all of them
// went. It waits for room no longer than patience. A terminal that is
// no longer there marks the strip gone.
func (made *strip) say(words string) bool {
	left, until := []byte(words), time.Now().Add(patience)
	for len(left) > 0 {
		wrote, err := syscall.Write(made.out, left)
		if wrote > 0 {
			left = left[wrote:]
			continue
		}
		if err != nil && !errors.Is(err, syscall.EAGAIN) &&
			!errors.Is(err, syscall.EINTR) {
			made.gone = true
			return false
		}
		if time.Now().After(until) {
			return false
		}
		time.Sleep(breath)
	}
	return true
}
