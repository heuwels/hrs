package main

import (
	"flag"
	"reflect"
	"strings"
	"testing"
)

func TestParseFlagsAnywhere(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantText    string
		wantStrat   int64
		wantImport  bool
	}{
		{
			name:       "flags before positional",
			args:       []string{"-s", "9", "Merge", "2x", "Sentry", "upgrades"},
			wantText:   "Merge 2x Sentry upgrades",
			wantStrat:  9,
			wantImport: false,
		},
		{
			name:       "flags after positional (the bug)",
			args:       []string{"Merge 2x Sentry upgrades", "-s", "9"},
			wantText:   "Merge 2x Sentry upgrades",
			wantStrat:  9,
			wantImport: false,
		},
		{
			name:       "flags split across positional",
			args:       []string{"-i", "Beko", "promotion", "-s", "7", "front-end"},
			wantText:   "Beko promotion front-end",
			wantStrat:  7,
			wantImport: true,
		},
		{
			name:       "no flags",
			args:       []string{"Firm", "up", "budget", "figures"},
			wantText:   "Firm up budget figures",
			wantStrat:  0,
			wantImport: false,
		},
		{
			name:       "only flags",
			args:       []string{"-s", "5", "-i"},
			wantText:   "",
			wantStrat:  5,
			wantImport: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			strategy := fs.Int64("s", 0, "")
			important := fs.Bool("i", false, "")
			pos := parseFlagsAnywhere(fs, tc.args)
			gotText := strings.Join(pos, " ")
			if gotText != tc.wantText {
				t.Errorf("text: got %q, want %q", gotText, tc.wantText)
			}
			if *strategy != tc.wantStrat {
				t.Errorf("strategy: got %d, want %d", *strategy, tc.wantStrat)
			}
			if *important != tc.wantImport {
				t.Errorf("important: got %v, want %v", *important, tc.wantImport)
			}
		})
	}
}

func TestParseFlagsAnywhere_EmptyArgs(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Int64("s", 0, "")
	pos := parseFlagsAnywhere(fs, nil)
	if !reflect.DeepEqual(pos, []string(nil)) && len(pos) != 0 {
		t.Errorf("expected empty positional, got %v", pos)
	}
}
