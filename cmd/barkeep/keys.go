package main

import (
	"errors"
	"syscall"
	"time"

	"golang.org/x/term"
)

// The strip takes keys when it is run at a terminal: h has the hounds
// come and go, c lights the next colourway and C the one before, and q
// or ctrl-c leaves. A key changes the lamp that is burning and does not
// start it again: its wax stays where it was.

// A press is what a key asks of the strip.
type press int

const (
	// asksNothing is a key the strip has no use for.
	asksNothing press = iota
	// asksToLeave ends the strip.
	asksToLeave
	// asksForHounds has the hounds come, or go if they are here.
	asksForHounds
	// asksForNext lights the next colourway in play.
	asksForNext
	// asksForLast lights the colourway before.
	asksForLast
)

// ctrlC is the byte a terminal that is not handling signals sends for
// ctrl-c.
const ctrlC = 0x03

// pressed is what a key asks.
func pressed(key byte) press {
	switch key {
	case 'q', ctrlC:
		return asksToLeave
	case 'h':
		return asksForHounds
	case 'c':
		return asksForNext
	case 'C':
		return asksForLast
	}
	return asksNothing
}

// A glow is the lava lamp the strip is burning: the seed its wax is
// picked by, when it began, which of the colourways in play it is,
// whether the hounds run over it, and whether one has been lit at all.
type glow struct {
	seed   uint64
	began  time.Time
	way    int
	hounds bool
	lit    bool
}

// after is the lamp lit next, at a moment, with the hounds or without:
// new wax, in the colourway after this one's, or the first when none
// has been lit.
func (burning glow) after(now time.Time, hounds bool, count int) glow {
	next := glow{seed: uint64(now.UnixNano()), began: now,
		hounds: hounds, lit: true}
	if burning.lit {
		next.way = (burning.way + 1) % count
	}
	return next
}

// keyed is the lamp after a press, at a moment, among so many
// colourways in play. A lamp still playing keeps its wax and when it
// began, and only what was asked changes: the hounds come or go, or the
// colourway is the next or the last.
//
// With no lamp playing one is lit: with the hounds when they were asked
// for, and in the next or the last colourway when that was.
//
// A press that asks nothing of a lamp gives the lamp back as it was.
func (burning glow) keyed(asked press, playing bool, now time.Time,
	count int) glow {
	next := burning
	switch asked {
	case asksForHounds:
		next.hounds = !(playing && burning.hounds)
	case asksForNext:
		next.way = burning.stepped(1, count)
	case asksForLast:
		next.way = burning.stepped(count-1, count)
	default:
		return burning
	}
	if !playing {
		next.seed, next.began = uint64(now.UnixNano()), now
	}
	next.lit = true
	return next
}

// stepped is the colourway so many on from this lamp's, round the ones
// in play. A lamp never lit stays on the first: the first press shows
// it.
func (burning glow) stepped(by, count int) int {
	if !burning.lit {
		return 0
	}
	return (burning.way + by) % count
}

// effect is the lamp as the effect that draws it.
func (burning glow) effect(ink lamps) effect {
	if burning.hounds {
		return hounds(burning.seed, ink, shadedFor(burning.way))
	}
	return lava(burning.seed, ink, shadedFor(burning.way))
}

// keyWait is how long the strip waits before it looks for a key again,
// when the terminal had none: picked, shorter than a press is felt.
const keyWait = 50 * time.Millisecond

// typed is the keys pressed at the terminal the strip was started from,
// one at a time, and what puts the terminal back as it was. The
// terminal is set to hand each key over as it is pressed, and not to
// show it.
//
// Started from no terminal, no key ever comes and there is nothing to
// put back.
func typed() (<-chan byte, func()) {
	if !term.IsTerminal(syscall.Stdin) {
		return nil, func() {}
	}
	was, err := term.MakeRaw(syscall.Stdin)
	if err != nil {
		return nil, func() {}
	}
	keys := make(chan byte)
	go func() {
		for {
			key, more := nextKey()
			if !more {
				return
			}
			keys <- key
		}
	}()
	return keys, func() { _ = term.Restore(syscall.Stdin, was) }
}

// nextKey waits for one key from the terminal, and says whether there
// can be more. The terminal may be one the strip writes to without
// waiting, and then a read with no key ready comes straight back: it
// is asked again after keyWait.
func nextKey() (byte, bool) {
	one := make([]byte, 1)
	for {
		count, err := syscall.Read(syscall.Stdin, one)
		switch {
		case count == 1:
			return one[0], true
		case errors.Is(err, syscall.EAGAIN),
			errors.Is(err, syscall.EINTR):
			time.Sleep(keyWait)
		default:
			return 0, false
		}
	}
}
