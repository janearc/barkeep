package main

import (
	"testing"
	"time"
)

// marking is an effect that lasts so long, wants so many frames a
// second, and writes its mark and how long it has played into the first
// cell's ink each time it is laid over a strip.
func marking(mark uint8, lasts time.Duration, rate int, seen *[]uint8) effect {
	return effect{lasts: lasts, rate: rate, over: func(cells [][]cell, since time.Duration) {
		*seen = append(*seen, mark)
		cells[0][0].ink = lamps{mark, uint8(since / time.Second), 0}
	}}
}

// Acts staged together each play from when they begin for as long as
// they last; those playing are laid over the strip in the order given,
// each told how long it has played itself; the whole lasts until the
// last has ended, and wants the quickest rate of those then playing.
func TestStagedActsPlayInTheirTimesAndTheirOrder(t *testing.T) {
	var seen []uint8
	whole := staged([]act{
		{marking(1, 10*time.Second, 10, &seen), 2 * time.Second},
		{marking(2, 4*time.Second, 24, &seen), 0},
		{marking(3, 5*time.Second, 5, &seen), 9 * time.Second},
	})
	if whole.lasts != 14*time.Second {
		t.Fatalf("it lasts %v, not until its last act ends", whole.lasts)
	}
	for _, at := range []struct {
		since time.Duration
		laid  []uint8
		rate  int
		last  lamps
	}{
		{1 * time.Second, []uint8{2}, 24, lamps{2, 1, 0}},
		{3 * time.Second, []uint8{1, 2}, 24, lamps{2, 3, 0}},
		{6 * time.Second, []uint8{1}, 10, lamps{1, 4, 0}},
		{10 * time.Second, []uint8{1, 3}, 10, lamps{3, 1, 0}},
		{13 * time.Second, []uint8{3}, 5, lamps{3, 4, 0}},
	} {
		seen = nil
		cells := [][]cell{{{}}}
		whole.over(cells, at.since)
		if string(seen) != string(at.laid) || cells[0][0].ink != at.last || whole.paced(at.since) != at.rate {
			t.Errorf("%v in: laid %v, the last left %v, at %d a second; want %v, %v, %d",
				at.since, seen, cells[0][0].ink, whole.paced(at.since), at.laid, at.last, at.rate)
		}
	}
}

// The show is one lamp through every colourway, then the ramps, with
// the rain over them all: the lamp rises as the rain begins to fade,
// and the ramps as the lamp sinks.
func TestTheShowIsTheRainTheLampsAndTheRamps(t *testing.T) {
	whole := show(sampled(loop(), 64), knobs{speed: 0.02}, lamps{209, 198, 228}, 7)
	fades := rainDims + rainFills + scrambles
	ramps := fades + time.Duration(len(cardWays()))*showLamp - lavaSinks
	if want := ramps + showRamps; whole.lasts != want {
		t.Fatalf("the show lasts %v, want %v", whole.lasts, want)
	}
	// the rain alone is quick; a lamp alone is slow; and where the rain
	// fades over the first lamp the show wants the rain's frames
	for since, want := range map[time.Duration]int{
		rainDims / 2:                    rainRate,
		fades + time.Second:             rainRate,
		fades + rainFades + time.Second: lavaRate,
		ramps + showRamps/2:             lavaRate,
	} {
		if got := whole.paced(since); got != want {
			t.Errorf("%v into the show it wants %d frames a second, not %d", since, got, want)
		}
	}
	// over a banner, once the rain has gone, which hides every cell
	// while it falls, the show leaves every letter a letter to the end,
	// and colours the cells beside them when the ramps are up
	for _, since := range []time.Duration{fades + rainFades, fades + rainFades + 6*time.Second, ramps + showRamps/2, whole.lasts - time.Millisecond} {
		cells := bannerCells([2]int{12, 6}, []string{"ab", "cd"}, lamps{209, 198, 228})
		whole.over(cells, since)
		if !cells[2][5].words || !cells[3][6].words {
			t.Fatalf("%v into the show a letter is no longer words", since)
		}
	}
	lit := bannerCells([2]int{12, 6}, []string{"ab"}, lamps{209, 198, 228})
	whole.over(lit, ramps+showRamps/2)
	if lit[0][0].ink == black && lit[0][0].ground == black {
		t.Error("with the ramps up, a cell beside the banner is still black")
	}
}

// What is wanted a second is the ramp's own rate with nothing playing,
// an effect's one rate, or what a paced effect wants at that moment.
func TestTheFramesWantedFollowWhatIsPlaying(t *testing.T) {
	began := time.Date(2026, 11, 12, 19, 0, 0, 0, time.UTC)
	var playing played
	if got := playing.wants(12, began); got != 12 {
		t.Errorf("nothing playing wants %d", got)
	}
	playing.start(effect{lasts: time.Minute, rate: 24}, began)
	if got := playing.wants(12, began.Add(time.Second)); got != 24 {
		t.Errorf("an effect at 24 wants %d", got)
	}
	playing.start(effect{lasts: time.Minute, paced: func(since time.Duration) int {
		return int(since / time.Second)
	}}, began)
	if got := playing.wants(12, began.Add(7*time.Second)); got != 7 {
		t.Errorf("a paced effect seven seconds in wants %d", got)
	}
	// and one that wants none at a moment is drawn at the ramp's rate
	if got := playing.wants(12, began); got != 12 {
		t.Errorf("a paced effect wanting none wants %d", got)
	}
}
