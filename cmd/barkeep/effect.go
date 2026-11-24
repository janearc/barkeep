package main

import (
	"math"
	"time"
)

// An effect changes what a strip shows for a while: given every cell as
// the strip would draw it, and how long the effect has been playing, it
// changes the cells in place. It lasts so long and is then over, and
// the strip is what it would have been.
//
// an effect works from the cells alone and from the time, never from
// which strip it is, so every strip shows the same thing at a moment.
//
// it is drawn so many frames a second while it plays: a scramble wants
// more than slow wax does.
//
// paced, where an effect has one, is how many frames a second it wants
// so long into its playing, in place of its one rate: a show of several
// effects wants what the one playing wants.
type effect struct {
	lasts time.Duration
	rate  int
	paced func(since time.Duration) int
	over  func(cells [][]cell, since time.Duration)
}

// played is the effect that is playing, if one is, and when it began.
type played struct {
	now   effect
	began time.Time
	live  bool
}

// start plays an effect from a moment, in place of any that is playing.
func (playing *played) start(next effect, at time.Time) {
	playing.now, playing.began, playing.live = next, at, true
}

// A stage is what plays over the strip: the effect playing, the lamp
// burning when the effect is a lamp, and the ink the readings are in.
type stage struct {
	playing played
	burning glow
	lamped  bool
	ink     lamps
}

// light plays a lamp in place of whatever is playing.
func (on *stage) light(next glow) {
	on.burning, on.lamped = next, true
	on.playing.start(next.effect(on.ink), next.began)
}

// put plays an effect that is no lamp, from a moment.
func (on *stage) put(next effect, at time.Time) {
	on.lamped = false
	on.playing.start(next, at)
}

// press does what a key asked of the lamp, at a moment: the lamp that
// is playing changed, or with none playing a lamp lit.
func (on *stage) press(asked press, now time.Time) {
	burning := on.lamped && on.playing.live
	next := on.burning.keyed(asked, burning, now, len(inPlay))
	if next != on.burning {
		on.light(next)
	}
}

// pause is how long to wait for the next frame: the ramp's own rate, or
// the effect's while one plays.
func (playing *played) pause(fps int) time.Duration {
	return time.Second / time.Duration(playing.wants(fps, time.Now()))
}

// wants is how many frames a second are wanted at a moment: the ramp's
// own, or the effect's while one plays and wants any.
func (playing *played) wants(fps int, now time.Time) int {
	if !playing.live {
		return fps
	}
	rate := playing.now.rate
	if playing.now.paced != nil {
		rate = playing.now.paced(now.Sub(playing.began))
	}
	if rate > 0 {
		return rate
	}
	return fps
}

// An overlay is an effect at one moment of its playing: what a strip
// applies to its cells before it draws them. The empty overlay changes
// nothing.
type overlay struct {
	of    effect
	since time.Duration
	live  bool
}

// at is the effect as it stands at a moment, and ends it when its time
// is up.
func (playing *played) at(now time.Time) overlay {
	if !playing.live {
		return overlay{}
	}
	since := now.Sub(playing.began)
	if since >= playing.now.lasts {
		playing.live = false
		return overlay{}
	}
	return overlay{of: playing.now, since: since, live: true}
}

// change applies the overlay to a strip's cells.
func (over overlay) change(cells [][]cell) {
	if over.live {
		over.of.over(cells, over.since)
	}
}

// The rain: the whole strip fades to black at once, fills as quickly as
// it can with bright green scrambled punctuation, and then over a few
// seconds the glyphs fade, leaving the readings that were behind them,
// while the ramps fade in as the green fades out.
const (
	// picked: the fade to black, quick
	rainDims = 200 * time.Millisecond
	// picked: seven frames to fill; fewer reads as a cut
	rainFills = 300 * time.Millisecond
	// picked: a few seconds
	rainFades = 3500 * time.Millisecond
	// picked: how long a scrambled glyph stays one glyph
	rainTurns = 70 * time.Millisecond
	// picked: frames a second; a scramble wants them
	rainRate = 24
	// picked: how far through the fade the last of the readings has
	// come out from behind its glyph; the rest of the fade it only
	// changes ink
	rainOut = 0.6
	// picked: a glyph's shimmer runs from its dimmest green to its
	// palest and back between once and twice a second, each at its own
	// rate
	shimmerSlow, shimmerFast = 1.0, 2.0
	// picked: the share of a shimmer spent between the dim green and
	// the bright; the rest is the flash toward the pale
	shimmerBody = 0.75
)

// scrambles is how long the rain stays all green before it fades, the
// -scramble flag.
var scrambles = 400 * time.Millisecond

// The rain's greens, picked: a glyph shimmers from the dim one through
// the bright one to the pale one and back.
var (
	rainDim   = lamps{0, 96, 28}
	rainGreen = lamps{0, 255, 70}
	rainPale  = lamps{176, 255, 196}
)

// black is no light.
var black = lamps{}

// punctuation is what the rain is made of.
var punctuation = []rune("!@#$%^&*()-_=+[]{};:'\",.<>/?\\|~`")

// rain is the rain as an effect, its scramble picked by a seed. It
// stays all green for as long as scrambles says when it starts.
func rain(seed uint64) effect {
	holds := scrambles
	return effect{lasts: rainDims + rainFills + holds + rainFades,
		rate: rainRate,
		over: func(cells [][]cell, since time.Duration) {
			for row := range cells {
				for column := range cells[row] {
					place := drop{seed, uint64(row),
						uint64(column), holds}
					cells[row][column] = place.at(
						cells[row][column], since)
				}
			}
		}}
}

// A drop is one cell's place in the rain: the rain's seed and the
// cell's row and column, from which everything that is its own comes,
// and how long the rain stays all green.
type drop struct {
	seed, row, column uint64
	holds             time.Duration
}

// shade is the green the cell's glyph is at a time: it shimmers from
// the dim green up through the bright to a flash of the pale and back,
// at a rate and from a start that are the cell's own, so no two cells
// keep time.
func (place drop) shade(since time.Duration) lamps {
	rate := shimmerSlow + (shimmerFast-shimmerSlow)*place.share(3)
	turn := since.Seconds()*rate + place.share(4)
	lit := 0.5 - 0.5*math.Cos(2*math.Pi*turn)
	if lit < shimmerBody {
		return between(rainDim, rainGreen, lit/shimmerBody)
	}
	return between(rainGreen, rainPale, (lit-shimmerBody)/(1-shimmerBody))
}

// share is a number from nought to one that is this cell's own, one for
// each use.
func (place drop) share(use uint64) float64 {
	return float64(mixedBits(place.seed^place.row<<20^place.column<<8^
		use<<40)>>11) / (1 << 53)
}

// glyph is the scrambled glyph the cell shows at a time.
func (place drop) glyph(since time.Duration) rune {
	turns := since / rainTurns
	if turns < 0 {
		turns = 0
	}
	turn := uint64(turns)
	bits := mixedBits(place.seed ^ place.row<<20 ^ place.column<<8 ^
		turn<<32)
	return punctuation[bits%uint64(len(punctuation))]
}

// at is what the cell shows so long into the rain, given what it would
// show without it.
func (place drop) at(was cell, since time.Duration) cell {
	green := cell{glyph: place.glyph(since), ink: place.shade(since),
		ground: black}
	switch {
	case since < rainDims:
		keep := 1 - float64(since)/float64(rainDims)
		was.ink, was.ground = between(black, was.ink, keep),
			between(black, was.ground, keep)
		return was
	case since < rainDims+rainFills:
		if place.share(1) < float64(since-rainDims)/float64(rainFills) {
			return green
		}
		return cell{glyph: ' ', ink: black, ground: black}
	case since < rainDims+rainFills+place.holds:
		return green
	}
	faded := float64(since-rainDims-rainFills-place.holds) /
		float64(rainFades)
	return place.fading(was, green, smooth(faded))
}

// fading is the cell a share of the way through the rain's fade. The
// ground comes up from black to the ramp's. A cell of the readings
// comes out from behind its glyph at a moment of its own and turns from
// green to its own ink; any other cell's glyph sinks into the ground.
//
// The green it fades from is the cell's shimmering one, so the shimmer
// goes on as it fades.
func (place drop) fading(was, green cell, faded float64) cell {
	if !was.words {
		under := mixed(was.ink, was.ground)
		ground := between(black, under, faded)
		green.ink = between(green.ink, ground, faded)
		green.ground = ground
		return green
	}
	ground := between(black, was.ground, faded)
	out := place.share(2) * rainOut
	if faded < out {
		green.ground = ground
		return green
	}
	was.ink = between(green.ink, was.ink, (faded-out)/(1-out))
	was.ground = ground
	return was
}

// between is a colour a share of the way from one to another, a lamp at
// a time.
func between(from, to lamps, share float64) lamps {
	share = min(max(share, 0), 1)
	mix := func(one, other uint8) uint8 {
		moved := (float64(other) - float64(one)) * share
		return uint8(float64(one) + moved + 0.5)
	}
	return lamps{mix(from[0], to[0]), mix(from[1], to[1]),
		mix(from[2], to[2])}
}

// smooth eases a share from nought to one: slow away, slow home.
func smooth(share float64) float64 {
	share = min(max(share, 0), 1)
	return share * share * (3 - 2*share)
}

// mixedBits scrambles 64 bits into 64: splitmix64's finisher, so near
// numbers give far ones.
func mixedBits(bits uint64) uint64 {
	bits += 0x9e3779b97f4a7c15
	bits = (bits ^ bits>>30) * 0xbf58476d1ce4e5b9
	bits = (bits ^ bits>>27) * 0x94d049bb133111eb
	return bits ^ bits>>31
}
