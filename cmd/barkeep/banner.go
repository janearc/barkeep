package main

import (
	"fmt"
	"os/exec"
	"strings"
)

// A banner is a word in large letters, alone in the middle of a strip:
// no loop under it and no readings beside it. It is for showing the
// effects in a pane of their own.
//
// The letters are figlet's, asked for once when the column starts.
// figlet is a program on this machine and not a library of ours: with
// no figlet, or no such font, there is no banner and the column says so
// and does not start.

// banner is the banner's lines as figlet drew them, the -banner flag's
// words in the -font flag's font. None, and the column is the loop and
// the readings.
var banner []string

// figlets is words in a figlet font, as lines: what figlet prints,
// without the empty lines round it or the spaces after each line.
//
// The letters are kerned, figlet's -k: set side by side with a space
// between, and not pushed into each other as figlet does unasked:
// letters that touch are harder to read.
func figlets(words, font string) ([]string, error) {
	drawn := exec.Command("figlet", "-f", font, "-k", "-w", "1000", words)
	out, err := drawn.Output()
	if err != nil {
		return nil, fmt.Errorf("figlet, for %q in %s: %w", words, font,
			err)
	}
	var lines []string
	for _, each := range strings.Split(string(out), "\n") {
		lines = append(lines, strings.TrimRight(each, " "))
	}
	for len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("figlet drew nothing for %q in %s",
			words, font)
	}
	return lines, nil
}

// bannerCells is every cell of a strip so many columns and rows that
// shows the banner alone: nothing, on black, and the banner's letters
// in the ink, in the middle.
//
// Nothing is a half block, black over black, and not a space: an effect
// that colours a cell colours its two dots, and a space has only its
// ground to show one of them.
//
// Its letters are words, so an effect leaves them to be read. A banner
// wider or taller than the strip is cut at the strip's edges, about its
// middle.
func bannerCells(size [2]int, lines []string, ink lamps) [][]cell {
	cells := make([][]cell, size[1])
	for row := range cells {
		cells[row] = make([]cell, size[0])
		for column := range cells[row] {
			cells[row][column] = cell{glyph: halfBlock, ink: black,
				ground: black}
		}
	}
	wide := 0
	for _, each := range lines {
		wide = max(wide, len([]rune(each)))
	}
	top, left := (size[1]-len(lines))/2, (size[0]-wide)/2
	for index, each := range lines {
		row := top + index
		if row < 0 || row >= size[1] {
			continue
		}
		for at, letter := range []rune(each) {
			column := left + at
			if letter == ' ' || column < 0 || column >= size[0] {
				continue
			}
			cells[row][column] = cell{glyph: letter, ink: ink,
				ground: black, words: true}
		}
	}
	return cells
}
