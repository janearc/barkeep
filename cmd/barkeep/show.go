package main

import "time"

// The show: every effect in turn, each fading into the next, for ever.
//
// It is the rain; under the rain as it fades, the lava lamp, one lamp
// whose wax turns through each colourway in turn; and under the lamp as
// it sinks, the ramps, the loop the column is named for. When the ramps
// have sunk it begins again.

// showing is whether the column plays the show: the -show flag.
var showing bool

// picked: how long the show's lamp is each colourway, the last of that
// spent turning into the next; and how long the ramps play after it.
const (
	showLamp  = 14 * time.Second
	showTurns = 5 * time.Second
	showRamps = 16 * time.Second
)

// An act is one effect of several played together, and how long after
// the first of them it begins.
type act struct {
	of     effect
	begins time.Duration
}

// staged is acts as one effect. Each plays from when it begins for as
// long as it lasts. Those playing at a moment are laid over the strip
// in the order they are given, the first undermost, so an effect given
// after another fades into it and not into the bare strip.
//
// It lasts until its last act has ended, and at each moment is drawn as
// often as the quickest of the acts then playing wants.
func staged(acts []act) effect {
	var lasts time.Duration
	for _, each := range acts {
		lasts = max(lasts, each.begins+each.of.lasts)
	}
	playing := func(each act, since time.Duration) bool {
		return since >= each.begins && since < each.begins+each.of.lasts
	}
	return effect{lasts: lasts,
		paced: func(since time.Duration) int {
			rate := 0
			for _, each := range acts {
				if playing(each, since) {
					rate = max(rate, each.of.rate)
				}
			}
			return rate
		},
		over: func(cells [][]cell, since time.Duration) {
			for _, each := range acts {
				if playing(each, since) {
					each.of.over(cells, since-each.begins)
				}
			}
		}}
}

// show is the show, its rain and its lamps picked by a seed: the lamps
// in every colourway there is, in their order, the first rising as the
// rain begins to fade and each after it as the one before begins to
// sink.
//
// The ramps rise as the last lamp sinks, and the rain is over them all,
// from the start.
//
// The lamps are one lamp, whose wax turns from each colourway to the
// next while its blobs go on as they were.
func show(round wheel, set knobs, ink lamps, seed uint64) effect {
	fades := rainDims + rainFills + scrambles
	lamp := lavaThrough(seed, ink, cardWays(), showLamp, showTurns)
	return staged([]act{
		{lamp, fades},
		{ramped(round, set, ink, showRamps),
			fades + lamp.lasts - lavaSinks},
		{rain(seed), 0},
	})
}

// ramped is the ramps as an effect: the loop running down the strip,
// turning, rising over whatever the strip shows and sinking away again
// as a lamp does. A cell of words keeps its glyph, on the colour
// between the loop's two there, in ink moved to read on it.
func ramped(round wheel, set knobs, ink lamps, lasts time.Duration) effect {
	return effect{lasts: lasts, rate: lavaRate,
		over: func(cells [][]cell, since time.Duration) {
			if len(cells) == 0 || len(cells[0]) == 0 {
				return
			}
			size := [2]int{len(cells[0]), len(cells)}
			at := moment{round, set.span,
				since.Seconds() * set.speed}
			if at.span <= 0 {
				at.span = round.fitted(size[1])
			}
			loop := loopAt(size, at, written{ink: ink})
			up := risen(since, lasts)
			for row := range cells {
				for column, was := range cells[row] {
					cells[row][column] = risenOver(was,
						loop[row][column], up, ink)
				}
			}
		}}
}

// risenOver is a cell with the loop's cell risen so far over it. A cell
// of words keeps its glyph: its ground goes toward the colour between
// the loop's two there, and its ink is moved to read on that. Any other
// cell goes toward the loop's own ink and ground.
func risenOver(was, under cell, up float64, ink lamps) cell {
	if was.words {
		middle := mixed(under.ink, under.ground)
		was.ground = between(was.ground, middle, up)
		was.ink = inkOn(ink, was.ground)
		return was
	}
	was.ink = between(was.ink, under.ink, up)
	was.ground = between(was.ground, under.ground, up)
	return was
}
