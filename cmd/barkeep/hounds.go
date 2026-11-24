package main

import (
	"math"
	"time"
)

// The hounds: a moving field of 🐶 🐕 🌭, flowing up and down the way
// an old screen saver's toasters flew. They drift over a lava lamp,
// which keeps glowing behind them, in lanes two cells wide down the
// strip. Lanes take turns going down and up, each hound at its own pace.
//
// They never cover a reading, so the strip can still be read through
// them. They play on SIGINFO, or open the strip with --hounds.
const (
	// picked: frames a second; a hound moves a row at a time
	houndsRate = 12
	// picked: rows a second, the slowest hound and the fastest
	houndSlowest, houndFastest = 1.5, 4.0
	// picked: rows of the strip for each hound in a lane, so a lane of
	// twelve rows holds two
	houndRoom = 6
	// laneStep is how far one lane's left edge is from the next's: a
	// hound's two cells and one cell of lamp between lanes
	laneStep = 3
)

// houndsFirst opens the column with the hounds instead of the rain, the
// -hounds flag.
var houndsFirst bool

// houndGlyphs is what the field is made of, each drawn two cells wide.
var houndGlyphs = []rune{'🐶', '🐕', '🌭'}

// hound is one runner: the lane it is in, where it starts, how fast it
// goes and which way, and which glyph it is the first time down.
type hound struct {
	lane  int
	start float64 // rows, at the effect's start
	pace  float64 // rows a second, always above zero
	down  bool
	first int // index into houndGlyphs on its first pass
}

// hounds is the field over a lava lamp, in the shades the lamp is given
// for each moment, for as long as a lava lamp plays.
func hounds(seed uint64, ink lamps,
	shaded func(time.Duration) []lamps) effect {
	return houndsFor(seed, ink, shaded, lavaLasts)
}

// houndsFor is hounds for a set length of time.
func houndsFor(seed uint64, ink lamps,
	shaded func(time.Duration) []lamps, lasts time.Duration) effect {
	lamp := lavaShaded(seed, ink, lasts, shaded)
	return effect{lasts: lasts, rate: houndsRate,
		over: func(cells [][]cell, since time.Duration) {
			lamp.over(cells, since)
			packed := pack(seed, len(cells), lanes(cells))
			runAll(cells, packed, since)
		}}
}

// lanes is how many lanes fit across a strip, each two cells wide.
func lanes(cells [][]cell) int {
	if len(cells) == 0 || len(cells[0]) < 2 {
		return 0
	}
	return (len(cells[0])-2)/laneStep + 1
}

// pack is the hounds for a strip of so many rows and lanes, picked by a
// seed: the same seed and size is always the same pack.
func pack(seed uint64, rows, count int) []hound {
	each := max(1, rows/houndRoom)
	bits := mixedBits(seed)
	var all []hound
	for lane := range count {
		for place := range each {
			bits = mixedBits(bits + 1)
			pace := houndSlowest + float64(bits%1000)/1000*
				(houndFastest-houndSlowest)
			all = append(all, hound{lane: lane,
				start: float64(place*rows) / float64(each),
				pace:  pace, down: lane%2 == 0,
				first: int(bits>>20) % len(houndGlyphs)})
		}
	}
	return all
}

// runAll draws every hound where it is at a moment.
func runAll(cells [][]cell, pack []hound, since time.Duration) {
	for _, each := range pack {
		row, glyph := each.at(len(cells), since)
		run(cells, row, each.lane*laneStep, glyph)
	}
}

// at is the row a hound is on at a moment and its glyph. A hound goes
// off one end and comes back at the other, one row out of sight between,
// and is the next glyph each time it comes back.
func (one hound) at(rows int, since time.Duration) (int, rune) {
	loop := float64(rows + 1)
	gone := one.start + one.pace*since.Seconds()
	passes := int(math.Floor(gone / loop))
	where := gone - float64(passes)*loop
	row := int(where)
	if !one.down {
		row = rows - 1 - row
	}
	return row, houndGlyphs[(one.first+passes)%len(houndGlyphs)]
}

// run puts a glyph two cells wide at a row and column, on the ground the
// lamp shows there. It leaves the cells alone where the row is off the
// strip, where the glyph would run past the edge, or where either cell
// is one of the readings.
func run(cells [][]cell, row, column int, glyph rune) {
	if row < 0 || row >= len(cells) || column < 0 ||
		column+1 >= len(cells[row]) {
		return
	}
	left, right := cells[row][column], cells[row][column+1]
	if left.words || right.words {
		return
	}
	ground := mixed(left.ink, left.ground)
	cells[row][column] = cell{glyph: glyph, ink: ground, ground: ground,
		wide: true}
	cells[row][column+1] = cell{ink: ground, ground: ground, under: true}
}
