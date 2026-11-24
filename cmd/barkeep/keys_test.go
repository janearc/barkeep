package main

import (
	"testing"
	"time"
)

// Each key asks its one thing, and a key the strip has no use for asks
// nothing.
func TestEachKeyAsksItsOneThing(t *testing.T) {
	for key, want := range map[byte]press{'q': asksToLeave,
		ctrlC: asksToLeave, 'h': asksForHounds, 'c': asksForNext,
		'C': asksForLast, 'x': asksNothing, ' ': asksNothing,
		'H': asksNothing, 0x1b: asksNothing} {
		if got := pressed(key); got != want {
			t.Errorf("the key %q asks %d, want %d", key, got, want)
		}
	}
}

// the moments the lamps of these tests are lit and pressed at
var (
	lampLit   = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	keyStruck = lampLit.Add(7 * time.Second)
)

// A lamp that is playing keeps its wax and its beginning through every
// key: only what was asked changes.
func TestAKeyChangesALampAndDoesNotStartItAgain(t *testing.T) {
	burning := glow{seed: 9, began: lampLit, way: 2, lit: true}
	for _, each := range []struct {
		asked press
		want  glow
	}{
		{asksForHounds, glow{9, lampLit, 2, true, true}},
		{asksForNext, glow{9, lampLit, 3, false, true}},
		{asksForLast, glow{9, lampLit, 1, false, true}},
		{asksNothing, burning},
		{asksToLeave, burning},
	} {
		if got := burning.keyed(each.asked, true, keyStruck, 5); got != each.want {
			t.Errorf("after press %d the lamp is %+v, want %+v",
				each.asked, got, each.want)
		}
	}
}

// The hounds key takes the hounds away from a lamp that has them.
func TestTheHoundsKeyTakesTheHoundsAway(t *testing.T) {
	burning := glow{seed: 9, began: lampLit, way: 2, hounds: true, lit: true}
	got := burning.keyed(asksForHounds, true, keyStruck, 5)
	if got.hounds || got.seed != 9 || got.began != lampLit {
		t.Fatalf("the lamp after the hounds key is %+v", got)
	}
}

// The colourways go round: next after the last in play is the first,
// and last before the first is the last in play.
func TestTheColourwayKeysGoRound(t *testing.T) {
	last := glow{way: 4, lit: true}
	if got := last.keyed(asksForNext, true, keyStruck, 5).way; got != 0 {
		t.Errorf("next after the last of five is %d, want 0", got)
	}
	first := glow{way: 0, lit: true}
	if got := first.keyed(asksForLast, true, keyStruck, 5).way; got != 4 {
		t.Errorf("last before the first of five is %d, want 4", got)
	}
}

// With no lamp playing a key lights one, new wax from the moment of the
// key: the hounds key with the hounds, a colourway key in the colourway
// asked and with the hounds as the last lamp had them.
func TestAKeyWithNoLampPlayingLightsOne(t *testing.T) {
	spent := glow{seed: 9, began: lampLit, way: 2, hounds: true, lit: true}
	seed := uint64(keyStruck.UnixNano())
	hounds := spent.keyed(asksForHounds, false, keyStruck, 5)
	if want := (glow{seed, keyStruck, 2, true, true}); hounds != want {
		t.Errorf("the hounds key lit %+v, want %+v", hounds, want)
	}
	next := spent.keyed(asksForNext, false, keyStruck, 5)
	if want := (glow{seed, keyStruck, 3, true, true}); next != want {
		t.Errorf("the next key lit %+v, want %+v", next, want)
	}
}

// Before any lamp has been lit, a colourway key lights the first, and
// the hounds key lights the first with the hounds.
func TestTheFirstKeyLightsTheFirstColourway(t *testing.T) {
	seed := uint64(keyStruck.UnixNano())
	for asked, hounds := range map[press]bool{asksForNext: false,
		asksForLast: false, asksForHounds: true} {
		got := glow{}.keyed(asked, false, keyStruck, 5)
		if want := (glow{seed, keyStruck, 0, hounds, true}); got != want {
			t.Errorf("press %d on no lamp lit %+v, want %+v", asked,
				got, want)
		}
	}
}

// A lamp lit after another is the next colourway round, with new wax;
// the first lamp lit is the first colourway.
func TestEachLampLitIsTheNextColourway(t *testing.T) {
	first := glow{}.after(lampLit, true, 3)
	if want := (glow{uint64(lampLit.UnixNano()), lampLit, 0, true, true}); first != want {
		t.Fatalf("the first lamp is %+v, want %+v", first, want)
	}
	second := first.after(keyStruck, false, 3)
	if second.way != 1 || second.hounds || second.began != keyStruck {
		t.Fatalf("the second lamp is %+v", second)
	}
	if round := (glow{way: 2, lit: true}).after(keyStruck, false, 3); round.way != 0 {
		t.Fatalf("the lamp after the last of three is colourway %d", round.way)
	}
}

// On the stage a key changes the lamp that is playing where it is: the
// effect began when the lamp did, not when the key was pressed.
func TestTheStageKeepsTheLampsBeginning(t *testing.T) {
	on := stage{ink: lamps{209, 198, 228}}
	on.light(glow{}.after(lampLit, false, len(inPlay)))
	on.press(asksForHounds, keyStruck)
	if !on.burning.hounds || on.playing.began != lampLit || !on.lamped {
		t.Fatalf("after the hounds key the stage is %+v, begun %v",
			on.burning, on.playing.began)
	}
	// a key that asks nothing leaves what is playing alone
	on.put(effect{lasts: time.Hour}, keyStruck)
	on.press(asksNothing, keyStruck)
	if on.lamped || on.playing.began != keyStruck {
		t.Fatal("a key that asks nothing changed what is playing")
	}
}

// With something that is no lamp playing, a key lights a new lamp from
// the moment of the key, and not the old one where it was.
func TestAKeyOverAnotherEffectLightsANewLamp(t *testing.T) {
	on := stage{ink: lamps{209, 198, 228}}
	on.light(glow{}.after(lampLit, false, len(inPlay)))
	on.put(effect{lasts: time.Hour}, lampLit.Add(time.Second))
	on.press(asksForNext, keyStruck)
	if !on.lamped || on.playing.began != keyStruck || on.burning.way != 1 {
		t.Fatalf("the stage is %+v, begun %v", on.burning, on.playing.began)
	}
}
