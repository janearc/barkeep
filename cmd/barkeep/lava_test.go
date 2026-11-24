package main

import (
	"testing"
	"time"
)

// A lamp the size of the one its sizes were picked in is grown by one;
// one of four times the area, whatever its shape, by two.
func TestALampIsGrownByItsArea(t *testing.T) {
	for _, each := range []struct{ wide, high, want float64 }{
		{lavaWide, lavaHigh, 1},
		{2 * lavaWide, 2 * lavaHigh, 2},
		{4 * lavaWide, lavaHigh, 2},
		{40, 40, 2},
	} {
		if got := grown(each.wide, each.high); got != each.want {
			t.Errorf("a lamp %v by %v is grown by %v, want %v", each.wide, each.high, got, each.want)
		}
	}
}

// A lamp three times the size along each side, with its blobs at three
// times the places, is the same lamp larger: every dot of the small one
// is the shade of the dot three times as far across and down.
func TestALargerLampIsTheSameLampLarger(t *testing.T) {
	waxes := blobs(11)
	shades := colourways[0].shades
	small := lamp{wide: lavaWide, high: lavaHigh, grown: 1, waxes: waxes, shades: shades}
	large := lamp{wide: 3 * lavaWide, high: 3 * lavaHigh, grown: grown(3*lavaWide, 3*lavaHigh), waxes: waxes, shades: shades}
	for _, wax := range waxes {
		across, down := wax.at(17, small.wide, small.high)
		small.middles = append(small.middles, [2]float64{across, down})
		large.middles = append(large.middles, [2]float64{3 * across, 3 * down})
	}
	differ := 0
	for down := 0.5; down < lavaHigh; down += 1.5 {
		for across := 0.5; across < lavaWide; across++ {
			if small.shade(across, down) != large.shade(3*across, 3*down) {
				differ++
			}
		}
	}
	if differ != 0 {
		t.Errorf("%d dots of the larger lamp are not the smaller lamp's", differ)
	}
}

// A lamp playing through colourways is each one's own shades for most
// of its time, is half way to the next half way through the turning,
// is the next when its time is up, and stays the last one for good.
func TestALampTurnsFromOneColourwayToTheNext(t *testing.T) {
	ways := colourways[:3]
	each, turning := 10*time.Second, 4*time.Second
	same := func(got, want []lamps) bool {
		for index := range want {
			if got[index] != want[index] {
				return false
			}
		}
		return len(got) == len(want)
	}
	for since, way := range map[time.Duration]int{0: 0, 6 * time.Second: 0, 10 * time.Second: 1, 16 * time.Second: 1, 20 * time.Second: 2, 29 * time.Second: 2, time.Hour: 2} {
		if !same(shadesAt(ways, each, turning, since), ways[way].shades) {
			t.Errorf("%v in, the lamp is not colourway %d", since, way)
		}
	}
	half := shadesAt(ways, each, turning, 8*time.Second)
	for index := range half {
		if want := between(ways[0].shades[index], ways[1].shades[index], 0.5); half[index] != want {
			t.Fatalf("half way through the turning, shade %d is %v, want %v", index, half[index], want)
		}
	}
	// and it is one lamp for as long as all its colourways
	if lasts := lavaThrough(3, lamps{}, ways, each, turning).lasts; lasts != 30*time.Second {
		t.Errorf("a lamp through three colourways of ten seconds lasts %v", lasts)
	}
}
