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

func TestLiquidationViewHasNoTerrainOrLand(t *testing.T) {
	s := newSolver(80, 40)
	s.initializeGalaxy(galaxyLiving)
	s.beginLiquidation()
	if s.terrainHeight(.1) != s.terrainHeight(.9) || s.terrainSlope(.9) != 0 {
		t.Fatal("liquidation retained the wave scene's beach geometry")
	}
	s.p = nil // isolate anything painted independently of the liquid
	pixels, land := s.rasterLiquid(120, 60)
	for index := range pixels {
		if pixels[index] != 0 || land[index] != 0 {
			t.Fatalf("liquidation view painted terrain at pixel %d", index)
		}
	}
}

func TestLiquidationInitializesOneForceFieldAndEventuallySettles(t *testing.T) {
	s := newSolver(48, 24)
	s.initializeGalaxy(galaxyLiving)
	for frame := 0; frame < 30; frame++ {
		s.stepGalaxy(1.0 / 60)
	}
	positions := make([]point, len(s.p))
	for index, p := range s.p {
		positions[index] = point{x: p.x, y: p.y}
	}
	s.beginLiquidation()
	velocities := make([]point, len(s.p))
	differentDirections := 0
	for index, p := range s.p {
		if p.x != positions[index].x || p.y != positions[index].y {
			t.Fatal("liquidation moved a particle before physics began")
		}
		velocities[index] = point{x: p.vx, y: p.vy}
		if index > 0 {
			first := velocities[0]
			dot := (first.x*p.vx + first.y*p.vy) /
				(math.Hypot(first.x, first.y) * math.Hypot(p.vx, p.vy))
			if dot < .85 {
				differentDirections++
			}
		}
	}
	if differentDirections < len(s.p)/3 {
		t.Fatalf("force field moved too uniformly: %d/%d directions differ", differentDirections, len(s.p))
	}
	s.beginLiquidation()
	for index, p := range s.p {
		if p.vx != velocities[index].x || p.vy != velocities[index].y {
			t.Fatal("repeated liquidation initialized another random force field")
		}
	}

	settled := false
	for frame := 0; frame < liquidationMaxFrames; frame++ {
		if s.stepLiquidation(1.0 / 60) {
			settled = true
			break
		}
	}
	if !settled {
		t.Fatal("liquidation did not stop within its finite frame budget")
	}
	for _, p := range s.p {
		if p.vx != 0 || p.vy != 0 || math.IsNaN(p.x) || math.IsNaN(p.y) ||
			p.x < 0 || p.x > 1 || p.y < s.terrainHeight(p.x) || p.y > 1 {
			t.Fatalf("invalid settled liquid particle: %+v", p)
		}
	}
}

func TestGalaxyAndGatherShareTheSameBoundaryRaster(t *testing.T) {
	s := newSolver(80, 48)
	s.initializeGalaxy(galaxyClassic)
	s.sequence = experimentGalaxyFrames
	galaxy, _ := s.rasterGalaxy(80, 48)
	gather, _ := s.rasterGather(80, 48, 0)
	if !bytes.Equal(galaxy, gather) {
		t.Fatal("galaxy-to-gather boundary changed rendering modes")
	}
}

func TestLivingGalaxyRemainsStructuredAndFinite(t *testing.T) {
	s := newSolver(80, 48)
	s.initializeGalaxy(galaxyLiving)
	for frame := 0; frame < 30*60; frame++ {
		s.stepGalaxy(1.0 / 60)
	}
	roles := map[galaxyRole]int{}
	for index, p := range s.p {
		roles[s.galaxy[index].role]++
		if math.IsNaN(p.x) || math.IsNaN(p.y) || p.x < 0 || p.x > 1 || p.y < 0 || p.y > 1 {
			t.Fatalf("invalid living galaxy particle: %+v", p)
		}
	}
	if roles[galaxyCore] == 0 || roles[galaxyArm] == 0 || roles[galaxyKnot] == 0 || roles[galaxyHalo] == 0 {
		t.Fatalf("missing galaxy roles: %#v", roles)
	}
	raster, _ := s.rasterGalaxy(80, 48)
	levels := map[byte]bool{}
	for _, value := range raster {
		if value > 0 {
			levels[value/32] = true
		}
	}
	if len(levels) < 3 {
		t.Fatalf("living galaxy lacks tonal depth: %d levels", len(levels))
	}
}

func TestGalaxyEffectsParseAndCompose(t *testing.T) {
	effects := parseGalaxyEffects("nebula,starfield,shooting-stars,pulse,unknown")
	wanted := galaxyNebula | galaxyStarfield | galaxyShootingStars | galaxyPulse
	if effects != wanted {
		t.Fatalf("unexpected effects mask: got %d, want %d", effects, wanted)
	}

	plain := newSolver(80, 48)
	plain.initializeGalaxy(galaxyLiving)
	plainRaster, _ := plain.rasterGalaxy(80, 48)
	dressed := newSolver(80, 48)
	dressed.galaxyEffects = wanted
	dressed.initializeGalaxy(galaxyLiving)
	dressedRaster, _ := dressed.rasterGalaxy(80, 48)
	if bytes.Equal(plainRaster, dressedRaster) {
		t.Fatal("composed effects did not alter galaxy raster")
	}
}

func TestCometAndImpactAreDistinctAndBounded(t *testing.T) {
	s := newSolver(80, 48)
	s.initializeGalaxy(galaxyLiving)
	galaxy, _ := s.rasterGalaxy(80, 48)
	comet, _, accent := s.rasterComet(80, 48, .55)
	var tail, head bool
	for _, value := range accent {
		tail = tail || value == 128
		head = head || value == 255
	}
	if !tail || !head {
		t.Fatalf("comet accent mask lacks tail/head labels: tail=%v head=%v", tail, head)
	}
	if bytes.Equal(galaxy, comet) {
		t.Fatal("comet did not alter galaxy raster")
	}
	hold, _ := s.rasterGalaxyImpactHold(80, 48)
	if bytes.Equal(galaxy, hold) {
		t.Fatal("impact hold did not overexpose the frozen galaxy")
	}
	s.stepGalaxyImpact(1.0/60, .2)
	impact, _ := s.rasterGalaxyImpact(80, 48, .2)
	if bytes.Equal(galaxy, impact) {
		t.Fatal("impact did not alter galaxy raster")
	}
	for _, p := range s.p {
		if math.IsNaN(p.x) || math.IsNaN(p.y) || p.x < 0 || p.x > 1 || p.y < 0 || p.y > 1 {
			t.Fatalf("impact produced invalid particle: %+v", p)
		}
	}
}

func TestRecurringCometDelayStaysWithinTenToThirtySeconds(t *testing.T) {
	seen := map[int]bool{}
	for sample := 0; sample < 200; sample++ {
		delay := randomRecurringCometDelayFrames()
		if delay < recurringCometDelayMinFrames || delay > recurringCometDelayMaxFrames {
			t.Fatalf("recurring delay outside bounds: %d", delay)
		}
		seen[delay] = true
	}
	if len(seen) < 2 {
		t.Fatal("recurring comet delay did not vary")
	}
}

func TestRecurringCometCrossesTheRotatingLogo(t *testing.T) {
	source, err := loadLogoSource("../source.png")
	if err != nil {
		t.Fatal(err)
	}
	const width, height = 80, 48
	s := newSolver(width, height)
	s.setLogoTarget(source.target(width, height, 48, 40), width, height)
	logo, _, surfaces := s.rasterRotatingLogo(width, height, math.Pi/3)
	s.randomizeRecurringCometPath()
	comet, _, accent := s.rasterRecurringComet(width, height, .55, logo, surfaces)
	if bytes.Equal(logo, comet) {
		t.Fatal("recurring comet did not alter the projected logo")
	}
	var cometLabel, preservedSurface bool
	for _, label := range accent {
		cometLabel = cometLabel || label >= 128
		preservedSurface = preservedSurface || label == logoSurfaceFront || label == logoSurfaceEdge || label == logoSurfaceBack
	}
	if !cometLabel || !preservedSurface {
		t.Fatalf("combined accent mask lost comet or logo labels: comet=%v surface=%v", cometLabel, preservedSurface)
	}
}

func TestRecurringCometUsesRandomDirectionsAndHitsLogoCenter(t *testing.T) {
	source, err := loadLogoSource("../source.png")
	if err != nil {
		t.Fatal(err)
	}
	const width, height = 80, 48
	s := newSolver(width, height)
	s.setLogoTarget(source.target(width, height, 48, 40), width, height)
	starts := map[[2]int]bool{}
	for sample := 0; sample < 100; sample++ {
		s.randomizeRecurringCometPath()
		path := s.recurringCometPath
		if path.start.x >= 0 && path.start.x <= 1 && path.start.y >= 0 && path.start.y <= 1 {
			t.Fatalf("comet start is not outside viewport: %+v", path.start)
		}
		if math.Abs(path.impact.x-.5) > .02 || math.Abs(path.impact.y-.5) > .02 {
			t.Fatalf("comet misses logo center: %+v", path.impact)
		}
		dx, dy := path.start.x-path.impact.x, path.start.y-path.impact.y
		starts[[2]int{int(math.Round(dx * 10)), int(math.Round(dy * 10))}] = true
	}
	if len(starts) < 8 {
		t.Fatalf("comet directions did not vary enough: %d", len(starts))
	}
}

func TestRecurringImpactAndRegatherExcludeGalaxyBackdrop(t *testing.T) {
	source, err := loadLogoSource("../source.png")
	if err != nil {
		t.Fatal(err)
	}
	const width, height = 80, 48
	makeSolver := func(effects galaxyEffect) *solver {
		s := newSolver(width, height)
		s.initializeGalaxy(galaxyLiving)
		s.galaxyEffects = effects
		s.setLogoTarget(source.target(width, height, 48, 40), width, height)
		s.logoAngle = math.Pi / 4
		s.recurringCometPath = cometPath{impact: point{x: .5, y: .5}}
		s.placeParticlesOnRotatingLogo(s.logoAngle)
		s.stepLogoImpact(1.0/60, .4)
		return s
	}
	plain := makeSolver(0)
	dressed := makeSolver(galaxyNebula | galaxyStarfield | galaxyShootingStars | galaxyPulse)
	plainImpact, _ := plain.rasterLogoImpact(width, height, .4)
	dressedImpact, _ := dressed.rasterLogoImpact(width, height, .4)
	if !bytes.Equal(plainImpact, dressedImpact) {
		t.Fatal("galaxy effects leaked into recurring logo impact")
	}
	plainGather, _, _ := plain.rasterRotatingLogoRegather(width, height, .35, plain.logoAngle)
	dressedGather, _, _ := dressed.rasterRotatingLogoRegather(width, height, .35, dressed.logoAngle)
	if !bytes.Equal(plainGather, dressedGather) {
		t.Fatal("galaxy effects leaked into recurring logo reconstruction")
	}
}

func TestLogoParticlesCanBeDestroyedAndGatheredAgain(t *testing.T) {
	source, err := loadLogoSource("../source.png")
	if err != nil {
		t.Fatal(err)
	}
	const width, height = 80, 48
	s := newSolver(width, height)
	s.initializeGalaxy(galaxyLiving)
	s.setLogoTarget(source.target(width, height, 48, 40), width, height)
	s.logoAngle = math.Pi / 4
	s.recurringCometPath = cometPath{impact: point{x: .5, y: .5}}
	s.placeParticlesOnRotatingLogo(s.logoAngle)
	s.galaxyImpactApplied = false
	for frame := 0; frame < experimentImpactFrames; frame++ {
		s.stepLogoImpact(1.0/60, float64(frame)/float64(experimentImpactFrames-1))
	}
	for frame := 0; frame < experimentGatherFrames; frame++ {
		s.advanceLogoRotation(1.0 / 60)
		s.stepRotatingLogoGather(1.0/60, float64(frame)/float64(experimentGatherFrames-1), s.logoAngle)
	}
	gathered, _, _ := s.rasterRotatingGather(width, height, 1, s.logoAngle)
	projected, _, _ := s.rasterRotatingLogo(width, height, s.logoAngle)
	if !bytes.Equal(gathered, projected) {
		t.Fatal("destroyed logo did not recreate as the current rotating projection")
	}
	for _, particle := range s.p {
		if math.IsNaN(particle.x) || math.IsNaN(particle.y) || particle.x < 0 || particle.x > 1 || particle.y < 0 || particle.y > 1 {
			t.Fatalf("recurring cycle produced invalid particle: %+v", particle)
		}
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

func TestLogoRotationLingersAtFrontAndBack(t *testing.T) {
	s := newSolver(80, 48)
	s.logoAngle = 0
	s.advanceLogoRotation(.1)
	frontStep := s.logoAngle
	s.logoAngle = math.Pi / 2
	s.advanceLogoRotation(.1)
	edgeStep := s.logoAngle - math.Pi/2
	s.logoAngle = math.Pi
	s.advanceLogoRotation(.1)
	backStep := s.logoAngle - math.Pi
	if edgeStep < frontStep*4.9 || math.Abs(frontStep-backStep) > 1e-9 {
		t.Fatalf("rotation did not linger symmetrically: front=%f edge=%f back=%f", frontStep, edgeStep, backStep)
	}
}

func TestRotatingLogoKeepsCenterAndChangesProjection(t *testing.T) {
	projected, depth := projectLogoPoint(point{}, 0, math.Pi/3, 100)
	if projected != (point{}) || depth != 0 {
		t.Fatalf("center pivot moved: projected=%+v depth=%f", projected, depth)
	}

	source, err := loadLogoSource("../source.png")
	if err != nil {
		t.Fatal(err)
	}
	const width, height = 80, 48
	s := newSolver(width, height)
	s.setLogoTarget(source.target(width, height, 48, 40), width, height)
	front, frontLand, frontSurfaces := s.rasterRotatingLogo(width, height, 0)
	quarter, quarterLand, quarterSurfaces := s.rasterRotatingLogo(width, height, math.Pi/2)
	if len(front) != width*height || len(quarter) != len(front) ||
		len(frontLand) != len(front) || len(quarterLand) != len(front) ||
		len(frontSurfaces) != len(front) || len(quarterSurfaces) != len(front) {
		t.Fatal("rotating logo returned invalid raster dimensions")
	}
	if bytes.Equal(front, quarter) {
		t.Fatal("quarter turn did not alter the projected logo")
	}
	seenSurfaces := map[byte]bool{}
	for _, label := range quarterSurfaces {
		seenSurfaces[label] = true
	}
	if !seenSurfaces[logoSurfaceFront] || !seenSurfaces[logoSurfaceEdge] {
		t.Fatalf("angled projection lacks face/edge labels: %#v", seenSurfaces)
	}
	for _, raster := range [][]byte{front, quarter} {
		visible := 0
		for _, value := range raster {
			if value > 0 {
				visible++
			}
		}
		if visible == 0 {
			t.Fatal("3-D projection disappeared")
		}
	}
}

func TestRotatingGatherUsesTheCurrentProjectedLogo(t *testing.T) {
	source, err := loadLogoSource("../source.png")
	if err != nil {
		t.Fatal(err)
	}
	const width, height = 80, 48
	s := newSolver(width, height)
	s.initializeGalaxy(galaxyLiving)
	s.setLogoTarget(source.target(width, height, 48, 40), width, height)
	angle := math.Pi / 3
	_, _, initialSurfaces := s.rasterRotatingGather(width, height, 0, angle)
	for _, label := range initialSurfaces {
		if label != 0 {
			t.Fatal("surface glyph labels appeared before the projected logo")
		}
	}
	s.stepRotatingLogoGather(1.0/60, 1, angle)
	gather, _, gatherSurfaces := s.rasterRotatingGather(width, height, 1, angle)
	projected, _, projectedSurfaces := s.rasterRotatingLogo(width, height, angle)
	if !bytes.Equal(gather, projected) {
		t.Fatal("completed gather did not preserve the rotating projection")
	}
	for index, value := range projected {
		if value > 0 && gatherSurfaces[index] != projectedSurfaces[index] {
			t.Fatalf("visible projected surface changed at pixel %d", index)
		}
	}
	flat, _ := s.rasterLogo(width, height)
	if bytes.Equal(gather, flat) {
		t.Fatal("rotating gather fell back to the static logo")
	}
}

func TestRotatingLogoParticleProjectionIsFiniteAndBounded(t *testing.T) {
	source, err := loadLogoSource("../source.png")
	if err != nil {
		t.Fatal(err)
	}
	const width, height = 80, 48
	s := newSolver(width, height)
	s.setLogoTarget(source.target(width, height, 48, 40), width, height)
	s.placeParticlesOnRotatingLogo(math.Pi / 2)
	for _, particle := range s.p {
		if math.IsNaN(particle.x) || math.IsNaN(particle.y) || math.IsInf(particle.x, 0) ||
			math.IsInf(particle.y, 0) || particle.x < 0 || particle.x > 1 || particle.y < 0 || particle.y > 1 {
			t.Fatalf("invalid projected particle: %+v", particle)
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
