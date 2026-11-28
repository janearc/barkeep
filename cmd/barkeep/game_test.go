package main

import (
	"slices"
	"testing"
)

const boardExample = `mutants: 1 lane, 0 running, 0 waiting, at most 3
tests: 2 lanes, 1 running, 1 waiting, at most 3
  running: /a/project/slayer-rc019, pid 14383, running for 4m, about 57m left
  waiting: /a/bin/azazel execute --queue /a/queue 717 in /a/project/caprica, pid 14784, waiting for 1m
gpu: 1 lane, 0 running, 1 waiting, at most 1
  waiting: /a/project/lab, pid 14785, waiting for 2m
gomaxprocs: 2, from your game config
`

func TestGameBoardShowsEveryQueueAndJob(t *testing.T) {
	want := []line{
		{name: "game", reading: "run/q/open/max"},
		{name: "mutants", reading: "0/0/1/3"},
		{name: "tests", reading: "1/1/2/3"},
		{name: "gpu", reading: "0/1/1/1"},
		{name: "t> slayer-rc019", reading: "14383"},
		{name: "t+ azazel #717 caprica", reading: "14784"},
		{name: "g+ lab", reading: "14785"},
	}
	if got := parseGame(boardExample).gameLines(21, 24, 0); !slices.Equal(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	if got := string(lettersOf(21, want[5])); got != "t+ azazel #717  14784" {
		t.Fatalf("lost the render job number in the live strip width: %q", got)
	}
}

func TestGameBoardPagesJobsInSmallPanes(t *testing.T) {
	board := parseGame(boardExample)
	first := board.gameLines(13, 6, 0)
	second := board.gameLines(13, 6, gamePage)
	last := board.gameLines(13, 6, gamePage*2)
	if !slices.Equal(first[:4], second[:4]) || !slices.Equal(first[:4], last[:4]) {
		t.Fatal("paging changed the lane totals")
	}
	if first[4].reading != "1/3" || second[4].reading != "2/3" ||
		last[4].reading != "3/3" || first[5].reading != "14383" ||
		second[5].reading != "14784" || last[5].reading != "14785" {
		t.Fatalf("wrong pages: %#v, %#v, %#v", first[4:], second[4:], last[4:])
	}
	if got := string(lettersOf(13, first[1])); got != "mut   0/0/1/3" {
		t.Fatalf("wrong narrow lane total: %q", got)
	}
}

func TestPartialGameBoardIsUnknown(t *testing.T) {
	partial := `mutants: 1 lane, 0 running, 0 waiting, at most 3
tests: 1 lane, 0 running, 1 waiting, at most 3
gpu: 1 lane, 0 running, 0 waiting, at most 1
`
	want := []line{{name: "game", reading: unknown}}
	if got := parseGame(partial).gameLines(21, 20, 0); !slices.Equal(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
