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
	s.initializeGalaxy()
	s.beginLiquidation()
	for x := 0; x < s.nx; x++ {
		if s.solidCell(x, 0) {
			t.Fatal("liquidation retained a collision floor")
		}
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
	s.initializeGalaxy()
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
	if s.liquidationFrames >= liquidationMaxFrames {
		t.Fatal("liquidation relied on the safety timeout instead of draining through the open bottom")
	}
	if len(s.p) != 0 {
		t.Fatalf("%d liquid particles remained instead of falling out of view", len(s.p))
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

func TestGalaxyRemainsStructuredAndFinite(t *testing.T) {
	s := newSolver(80, 48)
	s.initializeGalaxy()
	for frame := 0; frame < 30*60; frame++ {
		s.stepGalaxy(1.0 / 60)
	}
	roles := map[galaxyRole]int{}
	for index, p := range s.p {
		roles[s.galaxy[index].role]++
		if math.IsNaN(p.x) || math.IsNaN(p.y) || p.x < 0 || p.x > 1 || p.y < 0 || p.y > 1 {
			t.Fatalf("invalid galaxy particle: %+v", p)
		}
	}
	if roles[galaxyCore] == 0 || roles[galaxyArm] == 0 || roles[galaxyKnot] == 0 ||
		roles[galaxyHalo] == 0 || roles[galaxyDust] == 0 {
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
		t.Fatalf("galaxy lacks tonal depth: %d levels", len(levels))
	}
}

func TestGalaxyUsesAStableDifferentialRotationCurve(t *testing.T) {
	inner := galaxyAngularSpeed(.06)
	middle := galaxyAngularSpeed(.24)
	outer := galaxyAngularSpeed(.44)
	if !(inner > middle && middle > outer) {
		t.Fatalf("rotation curve is not differential: inner=%f middle=%f outer=%f", inner, middle, outer)
	}
	for _, speed := range []float64{inner, middle, outer} {
		if math.IsNaN(speed) || math.IsInf(speed, 0) || speed <= 0 || speed > 1 {
			t.Fatalf("invalid rotation speed: %f", speed)
		}
	}
}

func TestNebulaSurvivesDiffuseArmCompositing(t *testing.T) {
	plain := newSolver(80, 48)
	plain.initializeGalaxy()
	plainRaster, _ := plain.rasterGalaxy(80, 48)

	nebula := newSolver(80, 48)
	nebula.galaxyEffects = galaxyNebula
	nebula.initializeGalaxy()
	nebulaRaster, _ := nebula.rasterGalaxy(80, 48)

	brighter := 0
	for index := range plainRaster {
		if nebulaRaster[index] > plainRaster[index] {
			brighter++
		}
	}
	if brighter < len(plainRaster)/50 {
		t.Fatalf("nebula was erased by arm compositing: only %d brighter pixels", brighter)
	}
}

func TestRasterOnlyBackdropLeavesBeforeParticleGather(t *testing.T) {
	plain := newSolver(80, 48)
	plain.initializeGalaxy()
	dressed := newSolver(80, 48)
	dressed.galaxyEffects = galaxyNebula | galaxyShootingStars
	dressed.initializeGalaxy()

	plainGather, _ := plain.rasterGather(80, 48, .4)
	dressedGather, _ := dressed.rasterGather(80, 48, .4)
	if !bytes.Equal(plainGather, dressedGather) {
		t.Fatal("raster-only backdrop remained after the shared particles began gathering")
	}
	visible := 0
	for _, value := range dressedGather {
		if value > 0 {
			visible++
		}
	}
	if visible == 0 {
		t.Fatal("shared galaxy particles disappeared with the backdrop")
	}
}

func TestDeepFieldStarsShareTheTransitionParticleSet(t *testing.T) {
	s := newSolver(80, 48)
	s.galaxyEffects = galaxyStarfield
	s.initializeGalaxy()
	background := make([]int, 0)
	for index, star := range s.galaxy {
		if star.role == galaxyBackground {
			background = append(background, index)
		}
	}
	if len(background) < len(s.p)/50 {
		t.Fatalf("too few persistent deep-field particles: %d/%d", len(background), len(s.p))
	}

	s.targets = make([]point, len(s.p))
	for index := range s.targets {
		s.targets[index] = point{x: .5, y: .5}
	}
	before := 0.0
	for _, index := range background {
		before += math.Hypot(s.p[index].x-.5, s.p[index].y-.5)
	}
	for frame := 0; frame < 30; frame++ {
		s.stepLogoGather(1.0/60, .65)
	}
	after := 0.0
	for _, index := range background {
		after += math.Hypot(s.p[index].x-.5, s.p[index].y-.5)
	}
	if after >= before*.85 {
		t.Fatalf("deep-field particles did not join gather: before=%f after=%f", before, after)
	}
}

func TestGalaxyEffectsParseAndCompose(t *testing.T) {
	effects := parseGalaxyEffects("nebula,starfield,shooting-stars,pulse,unknown")
	wanted := galaxyNebula | galaxyStarfield | galaxyShootingStars | galaxyPulse
	if effects != wanted {
		t.Fatalf("unexpected effects mask: got %d, want %d", effects, wanted)
	}

	plain := newSolver(80, 48)
	plain.initializeGalaxy()
	plainRaster, _ := plain.rasterGalaxy(80, 48)
	dressed := newSolver(80, 48)
	dressed.galaxyEffects = wanted
	dressed.initializeGalaxy()
	dressedRaster, _ := dressed.rasterGalaxy(80, 48)
	if bytes.Equal(plainRaster, dressedRaster) {
		t.Fatal("composed effects did not alter galaxy raster")
	}
}

func TestCometAndImpactAreDistinctAndBounded(t *testing.T) {
	s := newSolver(80, 48)
	s.initializeGalaxy()
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
		s.initializeGalaxy()
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
	s.initializeGalaxy()
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

func TestLogoAlphaFringeDoesNotSoftenEdgesOrLighting(t *testing.T) {
	const width, height = 64, 48
	soft := make([]byte, width*height)
	hard := make([]byte, len(soft))
	for y := 10; y < 38; y++ {
		for x := 12; x < 52; x++ {
			soft[y*width+x] = 48 // Faint source-image antialiasing fringe.
			if x >= 16 && x < 48 && y >= 14 && y < 34 {
				soft[y*width+x] = 160 // Solid opacity must not dim the material.
				hard[y*width+x] = 255
			}
		}
	}
	a, b := newSolver(width, height), newSolver(width, height)
	a.setLogoTarget(soft, width, height)
	b.setLogoTarget(hard, width, height)
	for _, angle := range []float64{0, .6, 1.3, math.Pi / 2, math.Pi} {
		actual, _, labels := a.rasterRotatingLogo(width, height, angle)
		expected, _, expectedLabels := b.rasterRotatingLogo(width, height, angle)
		if !bytes.Equal(actual, expected) || !bytes.Equal(labels, expectedLabels) {
			t.Fatalf("source alpha fringe softened the silhouette or lighting at %f", angle)
		}
	}
}

func TestSolidLogoRespondsToFixedSideLight(t *testing.T) {
	front := logoSolidShade(0, 0, 1, 0)
	facingLight := logoSolidShade(0, 0, 1, -.8)
	away := logoSolidShade(0, 0, 1, .8)
	if facingLight-front < .2 || front-away < .2 {
		t.Fatalf("face does not show angled-light contrast: facing=%f front=%f away=%f", facingLight, front, away)
	}
	if logoSolidShade(-1, 0, 0, 0)-logoSolidShade(1, 0, 0, 0) < .5 {
		t.Fatal("edges do not distinguish lit side from shadow side")
	}
	if math.Abs(logoSolidShade(0, 0, -1, math.Pi)-front) > 1e-12 {
		t.Fatal("front and reverse do not share the same world-space light")
	}
}

func TestCenterUsesSameExtrusionAndLightingAsLogo(t *testing.T) {
	const width, height = 96, 80
	s := newSolver(width, height)
	alpha := make([]byte, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if math.Hypot(float64(x)-47.5, float64(y)-39.5) <= 8 ||
				(((y >= 10 && y < 20) || (y >= 60 && y < 70)) && ((x >= 12 && x < 20) || (x >= 76 && x < 84))) {
				alpha[y*width+x] = 255
			}
		}
	}
	s.setLogoTarget(alpha, width, height)
	var frontWidth int
	for _, angle := range []float64{0, .6, math.Pi / 2, math.Pi} {
		pixels, _, surfaces := s.rasterRotatingLogo(width, height, angle)
		left, right := width, -1
		centerFace, centerEdge := false, false
		for y := 30; y < 50; y++ {
			for x := 25; x < 71; x++ {
				i := y*width + x
				if pixels[i] == 0 {
					continue
				}
				left, right = min(left, x), max(right, x)
				switch surfaces[i] {
				case logoSurfaceFront, logoSurfaceBack:
					centerFace = true
				case logoSurfaceEdge:
					centerEdge = true
				default:
					t.Fatalf("center has a non-logo surface label: %d", surfaces[i])
				}
			}
		}
		if right < left {
			t.Fatalf("center disappeared at %f", angle)
		}
		if angle == 0 {
			frontWidth = right - left + 1
			if !centerFace {
				t.Fatal("center has no front face")
			}
			center := pixels[39*width+47]
			arm := pixels[14*width+15]
			if center != arm {
				t.Fatalf("center and arms have different lighting: %d vs %d", center, arm)
			}
		}
		if angle == math.Pi/2 && (centerFace || !centerEdge || right-left+1 >= frontWidth) {
			t.Fatal("center did not rotate edge-on with the logo")
		}
	}
}

func TestRotatingSolidFaceHasNoInteriorCracks(t *testing.T) {
	const width, height = 96, 80
	s := newSolver(width, height)
	alpha := make([]byte, width*height)
	for y := 16; y < 64; y++ {
		for x := 20; x < 76; x++ {
			alpha[y*width+x] = 255
		}
	}
	s.setLogoTarget(alpha, width, height)
	for _, angle := range []float64{0, .2, .6, 1, 1.3, 2, 2.6, math.Pi} {
		pixels, _, _ := s.rasterRotatingLogo(width, height, angle)
		for y := 36; y < 44; y++ {
			first, last := -1, -1
			for x := 0; x < width; x++ {
				if pixels[y*width+x] > 16 {
					if first < 0 {
						first = x
					}
					last = x
				}
			}
			if first < 0 {
				t.Fatalf("solid face disappeared at angle %f", angle)
			}
			for x := first; x <= last; x++ {
				if pixels[y*width+x] <= 16 {
					t.Fatalf("interior crack at angle %f, dot (%d,%d)", angle, x, y)
				}
			}
		}
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
	if !seenSurfaces[logoSurfaceEdge] {
		t.Fatalf("edge-on projection lacks edge labels: %#v", seenSurfaces)
	}
	// An exactly edge-on plane has zero area, so it must not leave ghost
	// face splats. Check the visible face at an actual oblique angle instead.
	_, _, angledSurfaces := s.rasterRotatingLogo(width, height, math.Pi/3)
	angledLabels := map[byte]bool{}
	for _, label := range angledSurfaces {
		angledLabels[label] = true
	}
	if !angledLabels[logoSurfaceFront] || !angledLabels[logoSurfaceEdge] {
		t.Fatalf("angled projection lacks face/edge labels: %#v", angledLabels)
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
	s.initializeGalaxy()
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

func TestDynamicGatherUsesCurrentDebrisAndPreservesTargets(t *testing.T) {
	source, err := loadLogoSource("../source.png")
	if err != nil {
		t.Fatal(err)
	}
	const width, height = 80, 48
	s := newSolver(width, height)
	s.initializeGalaxy()
	s.setLogoTarget(source.target(width, height, 0, 0), width, height)
	beforeTargets := make(map[point]int, len(s.targets))
	for _, target := range s.targets {
		beforeTargets[target]++
	}
	// Mimic an asymmetric collision so route construction has real debris state
	// to consume rather than relying on the original deterministic assignment.
	for index := range s.p {
		s.p[index].x = math.Mod(float64(index)*.173, 1)
		s.p[index].y = math.Mod(float64(index)*.317, 1)
	}
	s.prepareDynamicGather()
	if len(s.gatherOrigins) != len(s.p) || len(s.gatherWaypoints) != len(s.p) || len(s.gatherDelays) != len(s.p) {
		t.Fatal("dynamic gather did not prepare a route for every particle")
	}
	distinctRoutes := 0
	for index, particle := range s.p {
		if s.gatherOrigins[index] != (point{x: particle.x, y: particle.y}) {
			t.Fatalf("route %d ignored its current debris position", index)
		}
		if math.Hypot(s.gatherWaypoints[index].x-particle.x, s.gatherWaypoints[index].y-particle.y) > .02 {
			distinctRoutes++
		}
		beforeTargets[s.targets[index]]--
	}
	if distinctRoutes < len(s.p)/2 {
		t.Fatalf("only %d/%d particles received a meaningful dynamic route", distinctRoutes, len(s.p))
	}
	for target, count := range beforeTargets {
		if count != 0 {
			t.Fatalf("dynamic matching changed target multiplicity for %+v by %d", target, count)
		}
	}
}

func TestInitialCometRandomizesCollisionParameters(t *testing.T) {
	s := newSolver(80, 48)
	seen := make(map[cometPath]bool)
	seenStyles := make(map[int]bool)
	seenDurations := make(map[int]bool)
	for iteration := 0; iteration < 16; iteration++ {
		s.randomizeInitialCometPath()
		path := s.initialPath()
		seen[path] = true
		if path.impact.x < .38 || path.impact.x > .62 || path.impact.y < .41 || path.impact.y > .61 {
			t.Fatalf("randomized impact left the visible galaxy body: %+v", path.impact)
		}
		if s.impactStrength < .78 || s.impactStrength > 1.33 || math.Abs(s.impactSpin) > .24 ||
			s.impactStyle < 0 || s.impactStyle >= 5 || s.currentImpactFrames() < 14 || s.currentImpactFrames() > 30 ||
			s.impactScale < .72 || s.impactScale > 1.34 || s.impactAspect < .58 || s.impactAspect > 1.63 {
			t.Fatalf("invalid collision parameters: strength=%f spin=%f style=%d duration=%d scale=%f aspect=%f",
				s.impactStrength, s.impactSpin, s.impactStyle, s.currentImpactFrames(), s.impactScale, s.impactAspect)
		}
		seenStyles[s.impactStyle] = true
		seenDurations[s.currentImpactFrames()] = true
	}
	if len(seen) < 2 || len(seenStyles) < 2 || len(seenDurations) < 2 {
		t.Fatalf("collision variation was not observable: paths=%d styles=%d durations=%d",
			len(seen), len(seenStyles), len(seenDurations))
	}
}

func TestImpactStylesProduceDifferentShockPatterns(t *testing.T) {
	const width, height = 80, 48
	base := make([]byte, width*height)
	renders := make(map[string]bool)
	for style := 0; style < 5; style++ {
		s := newSolver(width, height)
		s.impactStyle = style
		s.impactScale = 1
		s.impactAspect = .72
		s.impactDirection = point{x: math.Cos(.7), y: math.Sin(.7)}
		pixels, _ := s.rasterImpactOver(width, height, .52, append([]byte(nil), base...), point{x: .5, y: .5})
		renders[string(pixels)] = true
	}
	if len(renders) < 4 {
		t.Fatalf("five impact styles collapsed to only %d visible shock patterns", len(renders))
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
