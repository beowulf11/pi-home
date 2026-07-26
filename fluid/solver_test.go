package main

import "testing"

func TestRasterAndProgression(t *testing.T) {
	s := newSolver(48, 24)
	if len(s.p) == 0 {
		t.Fatal("no particles")
	}
	before := s.p[0]
	s.step(1.0 / 60)
	s.step(1.0 / 60)
	if s.sequence != 2 {
		t.Fatalf("sequence=%d", s.sequence)
	}
	if s.p[0] == before {
		t.Fatal("particle state did not progress")
	}
	r := s.raster(31, 17)
	if len(r) != 31*17 {
		t.Fatalf("raster length=%d", len(r))
	}
	nonzero := false
	for _, v := range r {
		if v > 0 {
			nonzero = true
			break
		}
	}
	if !nonzero {
		t.Fatal("empty raster")
	}
}

func TestOutputResizeDoesNotResetState(t *testing.T) {
	s := newSolver(40, 20)
	s.step(1.0 / 60)
	sequence := s.sequence
	particle := s.p[0]
	_ = s.raster(20, 10)
	_ = s.raster(80, 40)
	if s.sequence != sequence || s.p[0] != particle {
		t.Fatal("raster resize mutated simulation")
	}
}
