package main

import (
	"bytes"
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
	r, land := s.raster(31, 17)
	if len(r) != 31*17 || len(land) != len(r) {
		t.Fatalf("raster lengths pixels=%d land=%d", len(r), len(land))
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
	raster, _ := s.raster(160, 68)
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

func TestRasterFillsInvisibleBeachWithLand(t *testing.T) {
	s := newSolver(80, 40)
	s.p = nil // isolate the static terrain layer
	width, height := 120, 60
	raster, land := s.raster(width, height)
	leftFloor, leftLand, rightLand := 0, 0, 0
	for y := 0; y < height; y++ {
		for x := 0; x < 10; x++ {
			if raster[y*width+x] > 0 {
				leftFloor++
			}
			if land[y*width+x] > 0 {
				leftLand++
			}
			if land[y*width+width-1-x] > 0 {
				rightLand++
			}
		}
	}
	if leftFloor == 0 {
		t.Fatal("flat floor disappeared from the water silhouette")
	}
	if leftLand != 0 {
		t.Fatalf("flat ocean floor was incorrectly colored as land: %d pixels", leftLand)
	}
	if rightLand == 0 {
		t.Fatal("rising beach was not visibly filled")
	}
}

func TestLargeRasterKeepsWaterContinuous(t *testing.T) {
	s := newSolver(48, 24)
	raster, _ := s.raster(240, 120)
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
	_, _ = s.raster(20, 10)
	_, _ = s.raster(80, 40)
	if s.sequence != sequence || s.p[0] != particle {
		t.Fatal("raster resize mutated simulation")
	}
}

func TestGalaxyAndGatherShareTheSameBoundaryRaster(t *testing.T) {
	s := newSolver(80, 48)
	s.initializeGalaxy()
	s.sequence = experimentGalaxyFrames
	galaxy, _ := s.rasterGalaxy(80, 48)
	gather, _ := s.rasterGather(80, 48, 0)
	if !bytes.Equal(galaxy, gather) {
		t.Fatal("galaxy-to-gather boundary changed rendering modes")
	}
}

func TestLogoTargetRetainsAspectAcrossViewports(t *testing.T) {
	source, err := loadLogoSource("../source.png")
	if err != nil {
		t.Fatal(err)
	}
	for _, viewport := range [][2]int{{80, 48}, {170, 56}, {170, 180}, {300, 200}} {
		target := source.target(viewport[0], viewport[1], 0, 0)
		minX, minY, maxX, maxY := viewport[0], viewport[1], -1, -1
		for index, alpha := range target {
			if alpha == 0 {
				continue
			}
			x, y := index%viewport[0], index/viewport[0]
			minX, minY = min(minX, x), min(minY, y)
			maxX, maxY = max(maxX, x), max(maxY, y)
		}
		width, height := maxX-minX+1, maxY-minY+1
		if width <= 0 || height <= 0 || math.Abs(float64(width)/float64(height)-1) > .03 {
			t.Fatalf("viewport %v distorted target to %dx%d", viewport, width, height)
		}
	}
}

func TestFluidLogoGatherConvergesOnResponsiveTarget(t *testing.T) {
	source, err := loadLogoSource("../source.png")
	if err != nil {
		t.Fatal(err)
	}
	const width, height = 80, 48
	target := source.target(width, height, 0, 0)
	s := newSolver(width, height)
	s.setLogoTarget(target, width, height)
	if len(s.targets) != len(s.p) {
		t.Fatalf("targets=%d particles=%d", len(s.targets), len(s.p))
	}
	errorAt := func() float64 {
		total := 0.0
		for index, p := range s.p {
			target := s.targets[index]
			total += math.Hypot(p.x-target.x, p.y-target.y)
		}
		return total / float64(len(s.p))
	}
	before := errorAt()
	for frame := 0; frame < experimentGatherFrames; frame++ {
		s.stepLogoGather(1.0/60, float64(frame)/float64(experimentGatherFrames-1))
	}
	after := errorAt()
	if after >= before*.08 {
		t.Fatalf("gather did not converge enough: %.4f -> %.4f", before, after)
	}
	pixels, land := s.rasterLogo(width, height)
	for index := range target {
		if pixels[index] != target[index] || land[index] != 0 {
			t.Fatalf("settled raster differs at pixel %d", index)
		}
	}
}
