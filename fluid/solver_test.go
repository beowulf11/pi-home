package main

import (
	"math"
	"testing"
)

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

func TestWaveMakerPeriodicallyAddsEnergy(t *testing.T) {
	s := newSolver(48, 24)
	s.p = []particle{{x: .05, y: .3}}
	if !s.applyWaveMaker(1.0 / 60) {
		t.Fatal("wave maker did not activate at the beginning of its cycle")
	}
	if s.p[0].vx <= 0 || s.p[0].vy <= 0 {
		t.Fatalf("wave maker did not push particle: %+v", s.p[0])
	}
	before := s.p[0]
	s.sequence = wavePulseFrames
	if s.applyWaveMaker(1.0/60) || s.p[0] != before {
		t.Fatal("wave maker should rest between pulses")
	}
}

func TestBeachRisesOnRightAndReceivesBreakingWave(t *testing.T) {
	s := newSolver(100, 52)
	if s.terrainHeight(.5) >= s.terrainHeight(.85) || s.terrainHeight(.85) >= s.terrainHeight(1) {
		t.Fatal("right-hand beach does not rise toward the shoreline")
	}
	for frame := 0; frame < wavePeriodFrames; frame++ {
		s.step(1.0 / 60)
	}
	runup := 0
	for _, p := range s.p {
		if p.y < s.terrainHeight(p.x) {
			t.Fatalf("particle entered beach: %+v", p)
		}
		if p.x > .78 && p.y > waterSurface+.03 {
			runup++
		}
	}
	if runup == 0 {
		t.Fatal("incoming swell never ran up the beach")
	}
}

func TestWaterRemainsBoundedAndFiniteAcrossCycles(t *testing.T) {
	s := newSolver(48, 24)
	particleCount := len(s.p)
	for frame := 0; frame < wavePeriodFrames*3; frame++ {
		s.step(1.0 / 60)
	}
	if len(s.p) != particleCount {
		t.Fatalf("water particle count changed: %d -> %d", particleCount, len(s.p))
	}
	for _, p := range s.p {
		if math.IsNaN(p.x) || math.IsNaN(p.y) || math.IsNaN(p.vx) || math.IsNaN(p.vy) ||
			math.IsInf(p.x, 0) || math.IsInf(p.y, 0) || math.IsInf(p.vx, 0) || math.IsInf(p.vy, 0) ||
			p.x < 0 || p.x > 1 || p.y < s.terrainHeight(p.x) || p.y > 1 {
			t.Fatalf("invalid particle after sustained run: %+v", p)
		}
	}
	raster := s.raster(160, 68)
	visible := 0
	for _, value := range raster {
		if value > 0 {
			visible++
		}
	}
	if visible < len(raster)/10 {
		t.Fatalf("water visually collapsed after sustained run: %d/%d pixels", visible, len(raster))
	}
	for frame := 0; frame < wavePulseFrames/2; frame++ {
		s.step(1.0 / 60)
	}
	moving := 0
	for _, p := range s.p {
		if math.Hypot(p.vx, p.vy) > .03 {
			moving++
		}
	}
	if moving < len(s.p)/5 {
		t.Fatalf("periodic swell did not reactivate settled water: %d/%d particles moving", moving, len(s.p))
	}
}

func TestLargeRasterKeepsWaterContinuous(t *testing.T) {
	s := newSolver(48, 24)
	raster := s.raster(240, 120)
	nonzero := 0
	for _, value := range raster {
		if value > 0 {
			nonzero++
		}
	}
	if nonzero <= len(s.p)*9 {
		t.Fatalf("large raster is too sparse: %d pixels for %d particles", nonzero, len(s.p))
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
