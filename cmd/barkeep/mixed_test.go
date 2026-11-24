package main

import "testing"

// Halfway between two bytes is their sum halved, rounded down, for every
// pair of bytes there is.
func TestHalfwayIsTheSumHalvedForEveryPairOfBytes(t *testing.T) {
	for one := range 256 {
		for other := range 256 {
			got := halfway(uint8(one), uint8(other))
			if int(got) != (one+other)/2 {
				t.Fatalf("halfway between %d and %d is %d; want %d", one,
					other, got, (one+other)/2)
			}
		}
	}
}

// and a mixed colour is halfway, a lamp at a time.
func TestAMixedColourIsHalfwayALampAtATime(t *testing.T) {
	got := mixed(lamps{0, 255, 7}, lamps{255, 255, 8})
	if want := (lamps{127, 255, 7}); got != want {
		t.Errorf("mixed is %v; want %v", got, want)
	}
}
