package main

import (
	"strings"
	"testing"
)

// A banner's cells are nothing on black with its letters in the ink, in
// the middle; its letters are words and nothing else is; and a banner
// too large for its strip is cut at the edges, not wrapped.
func TestABannerStandsAloneInTheMiddle(t *testing.T) {
	ink := lamps{209, 198, 228}
	cells := bannerCells([2]int{9, 5}, []string{"ab", "c d"}, ink)
	if len(cells) != 5 || len(cells[0]) != 9 {
		t.Fatalf("%d rows of %d", len(cells), len(cells[0]))
	}
	// two lines in five rows start a row down; three wide in nine, three in
	want := map[[2]int]rune{{1, 3}: 'a', {1, 4}: 'b', {2, 3}: 'c', {2, 5}: 'd'}
	for row := range cells {
		for column, each := range cells[row] {
			// where there is no letter there is a half block, black
			// over black, for an effect to colour both dots of
			letter, isLetter := want[[2]int{row, column}]
			inked := ink
			if !isLetter {
				letter, inked = halfBlock, black
			}
			if each.glyph != letter || each.words != isLetter || each.ground != black || each.ink != inked {
				t.Errorf("row %d column %d: %q, words %v, on %v", row, column, each.glyph, each.words, each.ground)
			}
		}
	}
	cut := bannerCells([2]int{3, 1}, []string{"vwxyz", "12345", "abcde"}, ink)
	if got := string([]rune{cut[0][0].glyph, cut[0][1].glyph, cut[0][2].glyph}); got != "234" {
		t.Errorf("a banner five by three in a strip three by one shows %q, not its middle", got)
	}
}

// The letters are figlet's: the alligator font draws barkeep seven
// lines high, with no empty line round it and no spaces after a line. A
// font there is not, is an error that names it.
func TestTheLettersAreFiglets(t *testing.T) {
	lines, err := figlets("barkeep", "alligator")
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 7 {
		t.Fatalf("barkeep in alligator is %d lines high", len(lines))
	}
	for index, each := range lines {
		if each == "" && (index == 0 || index == len(lines)-1) || each != strings.TrimRight(each, " ") {
			t.Errorf("line %d: %q", index, each)
		}
	}
	if !strings.Contains(strings.Join(lines, ""), "#") {
		t.Error("alligator draws with hashes, and there are none")
	}
	// kerned, its letters stand apart: the word is 82 columns at its
	// widest with the spaces after each line taken off, measured with
	// figlet 2.2.5, where pushed into each other it is 76
	widest := 0
	for _, each := range lines {
		widest = max(widest, len([]rune(each)))
	}
	if widest != 82 {
		t.Errorf("barkeep in alligator, kerned, is %d columns wide, not 82", widest)
	}
	if _, err := figlets("barkeep", "no-such-font-as-this"); err == nil || !strings.Contains(err.Error(), "no-such-font-as-this") {
		t.Errorf("a font there is not: %v", err)
	}
}
