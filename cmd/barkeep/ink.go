package main

import (
	libcolour "github.com/janearc/libreadme/colour"
	"github.com/janearc/libreadme/profile"
)

// reading is libreadme's current reader profile: hers.
var reading = currentProfile()

// currentProfile is libreadme's profile of whoever reads this screen:
// how much contrast a letter needs to be read. With none there is
// nothing to draw readable words by, so it does not go on.
func currentProfile() profile.Profile {
	current, err := profile.Current()
	if err != nil {
		panic("barkeep: libreadme has no current profile: " +
			err.Error())
	}
	return current
}

// inkOn is the ink a letter is drawn in over one cell of the strip: the
// bar's ink when it reads against the colour under it with room to
// spare, and otherwise moved there by libreadme, its lightness alone.
//
// When no lightness of its hue gets there it is black or white,
// whichever reads more.
func inkOn(ink, ground lamps) lamps {
	band := libcolour.Band{Low: reading.Contrast.ChipMin + 1.5,
		High:   reading.Contrast.High,
		Centre: reading.Contrast.ChipMin + 3}
	inked, under := readerOf(ink), readerOf(ground)
	if band.Contains(libcolour.Contrast(inked, under)) {
		return ink
	}
	fitted, reached := band.Into(inked, []libcolour.RGB{under})
	if reached {
		return lamps{byteOf(fitted.R), byteOf(fitted.G),
			byteOf(fitted.B)}
	}
	black, white := libcolour.RGB{}, libcolour.RGB{R: 255, G: 255, B: 255}
	if libcolour.Contrast(black, under) > libcolour.Contrast(white, under) {
		return lamps{}
	}
	return lamps{255, 255, 255}
}

// readerOf is a colour as libreadme measures it, 0 to 255 a lamp.
func readerOf(colour lamps) libcolour.RGB {
	return libcolour.RGB{R: float64(colour[0]), G: float64(colour[1]),
		B: float64(colour[2])}
}

// byteOf is a lamp's level as a byte, rounded and held inside 0 to 255.
func byteOf(level float64) uint8 {
	return uint8(min(max(level+0.5, 0), 255))
}
