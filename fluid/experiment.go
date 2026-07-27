package main

import (
	"image/png"
	"math"
	"math/rand/v2"
	"os"
	"sort"
	"strings"
)

const (
	experimentGalaxyFrames        = 180
	experimentCometDelayMinFrames = 180
	experimentCometDelayMaxFrames = 360
	experimentCometFrames         = 48
	experimentImpactHoldFrames    = 1
	experimentImpactFrames        = 18
	experimentGatherFrames        = 210
	recurringCometDelayMinFrames  = 10 * 60
	recurringCometDelayMaxFrames  = 30 * 60
	liquidationMinFrames          = 120
	liquidationMaxFrames          = 420
	liquidationStableFrames       = 30
	liquidationStableSpeed        = .04
	logoRotationRadiansPerSecond  = 2 * math.Pi / 8
	logoSurfaceFront              = byte(32)
	logoSurfaceEdge               = byte(64)
	logoSurfaceBack               = byte(96)
)

type point struct{ x, y float64 }

type cometPath struct {
	start, control, impact point
}

var initialCometPath = cometPath{
	start: point{x: -.08, y: .91}, control: point{x: .18, y: .72}, impact: point{x: .5, y: .53},
}

type galaxyEffect uint8

const (
	galaxyNebula galaxyEffect = 1 << iota
	galaxyStarfield
	galaxyShootingStars
	galaxyPulse
)

func parseGalaxyEffects(value string) galaxyEffect {
	var effects galaxyEffect
	for _, name := range strings.Split(value, ",") {
		switch strings.TrimSpace(name) {
		case "nebula":
			effects |= galaxyNebula
		case "starfield":
			effects |= galaxyStarfield
		case "shooting-stars":
			effects |= galaxyShootingStars
		case "pulse":
			effects |= galaxyPulse
		}
	}
	return effects
}

type galaxyRole byte

const (
	galaxyCore galaxyRole = iota
	galaxyArm
	galaxyKnot
	galaxyHalo
	galaxyDust
	galaxyBackground
)

type galaxyParticle struct {
	role                                    galaxyRole
	radius, angle, angularSpeed             float64
	baseRadius, baseAngle, epicyclePhase    float64
	epicycleRate, twinklePhase, twinkleRate float64
	brightness                              byte
	splatScale                              float64
}

type logoSource struct {
	width, height          int
	minX, minY, maxX, maxY int
	alpha                  []byte
}

func loadLogoSource(path string) (*logoSource, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	image, err := png.Decode(file)
	if err != nil {
		return nil, err
	}
	bounds := image.Bounds()
	source := &logoSource{
		width: bounds.Dx(), height: bounds.Dy(),
		minX: bounds.Dx(), minY: bounds.Dy(), maxX: -1, maxY: -1,
		alpha: make([]byte, bounds.Dx()*bounds.Dy()),
	}
	for y := 0; y < source.height; y++ {
		for x := 0; x < source.width; x++ {
			_, _, _, alpha := image.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			value := byte(alpha >> 8)
			source.alpha[y*source.width+x] = value
			if value > 4 {
				source.minX = min(source.minX, x)
				source.minY = min(source.minY, y)
				source.maxX = max(source.maxX, x)
				source.maxY = max(source.maxY, y)
			}
		}
	}
	return source, nil
}

// target places the cropped square mark in the same responsive size tiers as
// the static intro, but inside the complete fluid viewport.
func (source *logoSource) target(width, height, requestedWidth, requestedHeight int) []byte {
	canvas := make([]byte, width*height)
	if source == nil || source.maxX < source.minX || width < 1 || height < 1 {
		return canvas
	}
	availableWidth, availableHeight := requestedWidth, requestedHeight
	if availableWidth <= 0 || availableHeight <= 0 {
		terminalRows := height / 2
		maxWidth, maxRows := 48, 20
		if width >= 220 && terminalRows >= 70 {
			maxWidth, maxRows = 96, 40
		} else if width >= 140 && terminalRows >= 52 {
			maxWidth, maxRows = 72, 30
		}
		availableWidth = min(maxWidth, int(float64(width)*.78))
		availableHeight = min(maxRows*2, height-4)
	}
	availableWidth = max(2, min(availableWidth, width))
	availableHeight = max(2, min(availableHeight, height))
	cropWidth := source.maxX - source.minX + 1
	cropHeight := source.maxY - source.minY + 1
	// Fit one axis, then derive the other from the source ratio. Never size the
	// axes independently: tall/narrow resizes must change scale, not geometry.
	sourceAspect := float64(cropWidth) / float64(cropHeight)
	targetWidth := availableWidth
	targetHeight := max(1, int(math.Round(float64(targetWidth)/sourceAspect)))
	if targetHeight > availableHeight {
		targetHeight = availableHeight
		targetWidth = max(1, int(math.Round(float64(targetHeight)*sourceAspect)))
	}
	scale := float64(targetWidth) / float64(cropWidth)
	offsetX := (width - targetWidth) / 2
	offsetY := (height - targetHeight) / 2
	for y := 0; y < targetHeight; y++ {
		sourceY := source.minY + min(cropHeight-1, int((float64(y)+.5)/scale))
		for x := 0; x < targetWidth; x++ {
			sourceX := source.minX + min(cropWidth-1, int((float64(x)+.5)/scale))
			canvas[(offsetY+y)*width+offsetX+x] = source.alpha[sourceY*source.width+sourceX]
		}
	}
	return canvas
}

func (s *solver) setLogoTarget(alpha []byte, width, height int) {
	if len(alpha) != width*height || len(s.p) == 0 {
		return
	}
	s.targetAlpha = append(s.targetAlpha[:0], alpha...)
	s.targetWidth, s.targetHeight = width, height
	total := uint64(0)
	for _, value := range alpha {
		total += uint64(value)
	}
	if total == 0 {
		return
	}
	targets := make([]point, len(s.p))
	cursor := 0
	cumulative := uint64(alpha[0])
	for index := range targets {
		wanted := (uint64(index)*total + total/2) / uint64(len(targets))
		for cursor < len(alpha)-1 && cumulative < wanted {
			cursor++
			cumulative += uint64(alpha[cursor])
		}
		x, y := cursor%width, cursor/width
		targets[index] = point{
			x: (float64(x) + .5) / float64(width),
			y: 1 - (float64(y)+.5)/float64(height),
		}
	}
	center := point{x: .5, y: .5}
	targetOrder := make([]int, len(targets))
	particleOrder := make([]int, len(s.p))
	for index := range targets {
		targetOrder[index], particleOrder[index] = index, index
	}
	angleRadius := func(p point) (float64, float64) {
		dx, dy := p.x-center.x, p.y-center.y
		return math.Atan2(dy, dx), math.Hypot(dx, dy)
	}
	sort.Slice(targetOrder, func(i, j int) bool {
		ai, ri := angleRadius(targets[targetOrder[i]])
		aj, rj := angleRadius(targets[targetOrder[j]])
		if math.Abs(ai-aj) > 1e-8 {
			return ai < aj
		}
		return ri < rj
	})
	sort.Slice(particleOrder, func(i, j int) bool {
		pi, pj := s.p[particleOrder[i]], s.p[particleOrder[j]]
		ai, ri := angleRadius(point{x: pi.x, y: pi.y})
		aj, rj := angleRadius(point{x: pj.x, y: pj.y})
		if math.Abs(ai-aj) > 1e-8 {
			return ai < aj
		}
		return ri < rj
	})
	s.targets = make([]point, len(s.p))
	s.gatherOrigins = nil
	for rank, particleIndex := range particleOrder {
		s.targets[particleIndex] = targets[targetOrder[rank]]
	}
}

func hashUnit(value int) float64 {
	x := uint32(value)*747796405 + 2891336453
	x = ((x >> ((x >> 28) + 4)) ^ x) * 277803737
	x = (x >> 22) ^ x
	return float64(x) / float64(^uint32(0))
}

// galaxyAngularSpeed approximates a flat outer rotation curve. A softened
// center avoids an unbounded angular velocity while the outer disk rotates
// more slowly, giving the arms subtle shear instead of rigidly spinning like a
// plate. This is the inexpensive visual analogue of a halo-supported curve.
func galaxyAngularSpeed(radius float64) float64 {
	return .14 + .105/math.Sqrt(math.Max(.035, radius)+.075)
}

// logarithmicSpiralAngle produces density-wave style arms. Logarithmic spirals
// keep a nearly constant pitch angle, unlike the previous tightly wound
// Archimedean placement whose pitch visibly changed across the disk.
func logarithmicSpiralAngle(radius float64) float64 {
	return 2.85 * math.Log(math.Max(.055, radius)/.055)
}

// beginLiquidation treats an invisible randomized force object as a spatial
// field, not as a visible projectile. Its offset center, travel bias, swirl and
// strength give every particle a related but different one-time velocity. The
// result is a coherent burst rather than either uniform motion or white noise.
func (s *solver) beginLiquidation() {
	if s.liquidationInitialized {
		return
	}
	s.liquidationInitialized = true
	s.liquidationFrames = 0
	s.stableFrames = 0
	centerX := .5 + (rand.Float64()-.5)*.24
	centerY := .53 + (rand.Float64()-.5)*.20
	travelAngle := rand.Float64() * 2 * math.Pi
	travelX, travelY := math.Cos(travelAngle), math.Sin(travelAngle)
	strength := .72 + rand.Float64()*.68
	swirl := .35 + rand.Float64()*.5
	if rand.Float64() < .5 {
		swirl = -swirl
	}
	for index := range s.p {
		p := &s.p[index]
		dx, dy := p.x-centerX, p.y-centerY
		distance := math.Max(.025, math.Hypot(dx, dy))
		normalX, normalY := dx/distance, dy/distance
		// Radial displacement varies across the silhouette, travel gives the
		// burst a shared gesture, and tangent motion bends it into a fluid arc.
		velocityX := normalX*.72 + travelX*.28 - normalY*swirl
		velocityY := normalY*.72 + travelY*.28 + normalX*swirl
		velocityLength := math.Hypot(velocityX, velocityY)
		velocityX, velocityY = velocityX/velocityLength, velocityY/velocityLength
		variation := (rand.Float64() - .5) * .42
		cosine, sine := math.Cos(variation), math.Sin(variation)
		velocityX, velocityY = velocityX*cosine-velocityY*sine, velocityX*sine+velocityY*cosine
		falloff := .55 + .45*math.Exp(-distance/.30)
		particleStrength := strength * falloff * (.78 + rand.Float64()*.44)
		p.vx = p.vx*.08 + velocityX*particleStrength
		p.vy = p.vy*.08 + velocityY*particleStrength
	}
}

// stepLiquidation returns true once the water has stayed quiet long enough.
// A finite upper bound guarantees that the intro cannot keep repainting due to
// tiny residual numerical velocities.
func (s *solver) stepLiquidation(dt float64) bool {
	s.beginLiquidation()
	s.stepPhysics(dt, false, .96)
	s.liquidationFrames++
	maximumSpeed := 0.0
	for index := range s.p {
		maximumSpeed = math.Max(maximumSpeed, math.Hypot(s.p[index].vx, s.p[index].vy))
	}
	if s.liquidationFrames >= liquidationMinFrames && maximumSpeed < liquidationStableSpeed {
		s.stableFrames++
	} else {
		s.stableFrames = 0
	}
	settled := s.stableFrames >= liquidationStableFrames || s.liquidationFrames >= liquidationMaxFrames
	if settled {
		for index := range s.p {
			s.p[index].vx = 0
			s.p[index].vy = 0
		}
	}
	return settled
}

// initializeGalaxy repurposes every fluid marker for the single current
// logarithmic density-wave simulation. Presets vary only transitions and
// effects; no legacy galaxy implementation remains selectable.
func (s *solver) initializeGalaxy() {
	if s.experimentInitialized {
		return
	}
	s.experimentInitialized = true
	s.galaxy = make([]galaxyParticle, len(s.p))
	coreCount := max(1, len(s.p)/6)
	for index := range s.p {
		randomA := hashUnit(index*7 + 1)
		randomB := hashUnit(index*7 + 2)
		randomC := hashUnit(index*7 + 3)
		star := galaxyParticle{
			twinklePhase: randomA * 2 * math.Pi,
			twinkleRate:  .7 + randomB*1.4,
			brightness:   byte(105 + int(randomC*145)),
			splatScale:   .75 + randomB*.65,
		}
		if index < coreCount {
			star.role = galaxyCore
			star.baseRadius = .09 * math.Sqrt(randomA)
			star.baseAngle = randomB * 2 * math.Pi
		} else {
			t := float64(index-coreCount) / float64(max(1, len(s.p)-coreCount-1))
			if s.galaxyEffects&galaxyStarfield != 0 && randomC < .08 {
				star.role = galaxyBackground
				star.baseRadius = .5
				star.baseAngle = randomB * 2 * math.Pi
				star.brightness = byte(68 + int(randomA*72))
				star.splatScale = .55
			} else if randomC < .12 {
				star.role = galaxyHalo
				star.baseRadius = .16 + .32*math.Sqrt(randomA)
				star.baseAngle = randomB * 2 * math.Pi
				star.brightness = byte(76 + int(randomC*390))
				star.splatScale = .62
			} else {
				star.role = galaxyArm
				if index%17 == 0 {
					star.role = galaxyKnot
					star.brightness = byte(225 + int(randomC*30))
					star.splatScale = 1.45
				} else if index%7 == 0 {
					star.role = galaxyDust
					star.brightness = byte(58 + int(randomC*75))
					star.splatScale = .72
				}
				star.baseRadius = .055 + .40*math.Pow(t, .58)
				armIndex := index % 4
				arm := float64(armIndex) * math.Pi / 2
				armWidth := .14 + .38*t
				if armIndex >= 2 {
					armWidth *= 1.35
					star.brightness = byte(float64(star.brightness) * .82)
				}
				jitter := (randomA - .5) * armWidth
				star.baseAngle = arm + logarithmicSpiralAngle(star.baseRadius) + jitter
				if star.role == galaxyDust {
					star.baseAngle -= .11
				}
			}
		}
		star.radius = star.baseRadius
		star.angle = star.baseAngle
		star.angularSpeed = galaxyAngularSpeed(star.baseRadius)
		star.epicyclePhase = randomC * 2 * math.Pi
		star.epicycleRate = .9 + randomA*1.25
		s.galaxy[index] = star
	}
	s.placeGalaxyParticles(0)
}

func (s *solver) placeGalaxyParticles(dt float64) {
	s.galaxyTime += dt
	tilt := .11
	for index := range s.p {
		star := &s.galaxy[index]
		oldX, oldY := s.p[index].x, s.p[index].y
		if star.role == galaxyBackground {
			baseX := .025 + .95*hashUnit(index*11+3101)
			baseY := .035 + .93*hashUnit(index*11+3102)
			s.p[index].x = baseX + .004*math.Sin(star.twinklePhase+s.galaxyTime*.17)
			s.p[index].y = baseY + .003*math.Cos(star.twinklePhase+s.galaxyTime*.13)
			if dt > 0 {
				s.p[index].vx = (s.p[index].x - oldX) / dt
				s.p[index].vy = (s.p[index].y - oldY) / dt
			}
			continue
		}
		breath := .010 * math.Sin(s.galaxyTime*1.15+star.epicyclePhase)
		epicycle := .020 * math.Sin(star.epicyclePhase+s.galaxyTime*star.epicycleRate)
		radius := star.baseRadius*(1+breath) + .004*math.Sin(star.epicyclePhase+s.galaxyTime*star.epicycleRate)
		angle := star.baseAngle + s.galaxyTime*star.angularSpeed + epicycle
		if star.role == galaxyHalo {
			angle = star.baseAngle + s.galaxyTime*(.12+.08*hashUnit(index+91))
		}
		diskX := radius * math.Cos(angle)
		diskY := radius * .72 * math.Sin(angle)
		s.p[index].x = .5 + diskX*math.Cos(tilt) - diskY*math.Sin(tilt)
		s.p[index].y = .53 + diskX*math.Sin(tilt) + diskY*math.Cos(tilt)
		if dt > 0 {
			s.p[index].vx = (s.p[index].x - oldX) / dt
			s.p[index].vy = (s.p[index].y - oldY) / dt
		}
	}
}

func (s *solver) stepGalaxy(dt float64) {
	s.placeGalaxyParticles(dt)
	s.sequence++
}

func screenBlendByte(base byte, light float64) byte {
	light = math.Max(0, math.Min(255, light))
	return byte(255 - (255-float64(base))*(255-light)/255)
}

func splatMaximum(out []byte, width, height int, cx, cy, radiusX, radiusY, brightness float64) {
	for y := int(math.Floor(cy - radiusY)); y <= int(math.Ceil(cy+radiusY)); y++ {
		for x := int(math.Floor(cx - radiusX)); x <= int(math.Ceil(cx+radiusX)); x++ {
			if x < 0 || y < 0 || x >= width || y >= height {
				continue
			}
			distance := math.Hypot((float64(x)-cx)/radiusX, (float64(y)-cy)/radiusY)
			value := byte(math.Max(0, math.Min(255, brightness*(1-math.Min(1, distance)))))
			if value > out[y*width+x] {
				out[y*width+x] = value
			}
		}
	}
}

func (s *solver) particleBrightness(index int) float64 {
	star := s.galaxy[index]
	amplitude := .04
	if star.role == galaxyKnot || star.role == galaxyHalo || star.role == galaxyBackground {
		amplitude = .18
	}
	return float64(star.brightness) * (1 - amplitude + amplitude*math.Sin(star.twinklePhase+s.galaxyTime*star.twinkleRate))
}

func (s *solver) rasterGalaxyBackdrop(out []byte, width, height int, intensity float64) {
	if s.galaxyEffects&galaxyNebula != 0 {
		// Broad clouds provide depth; an arm-aligned procedural field prevents the
		// nebula from reading as three unrelated circular blobs.
		breath := .88 + .12*math.Sin(s.galaxyTime*.43)
		clouds := []struct{ x, y, rx, ry, brightness float64 }{
			{.34, .43, .23, .19, 38}, {.63, .57, .28, .16, 32}, {.52, .31, .19, .13, 23},
		}
		for _, cloud := range clouds {
			splatMaximum(out, width, height,
				cloud.x*float64(width-1), cloud.y*float64(height-1),
				cloud.rx*float64(width), cloud.ry*float64(height),
				cloud.brightness*breath*intensity)
		}
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				dx := (float64(x)/float64(max(1, width-1)) - .5)
				dy := (float64(y)/float64(max(1, height-1)) - .47) / .72
				radius := math.Hypot(dx, dy)
				if radius < .055 || radius > .46 {
					continue
				}
				angle := math.Atan2(-dy, dx) - s.galaxyTime*.31
				wave := .5 + .5*math.Cos(2*(angle-logarithmicSpiralAngle(radius)))
				envelope := smoothstep((radius-.055)/.08) * (1 - smoothstep((radius-.34)/.12))
				noise := .72 + .28*hashUnit((x+1)*92821+(y+1)*68917)
				light := 38 * math.Pow(wave, 5) * envelope * noise * breath * intensity
				index := y*width + x
				out[index] = screenBlendByte(out[index], light)
			}
		}
	}
}

func (s *solver) rasterShootingStars(out []byte, width, height int, intensity float64) {
	if s.galaxyEffects&galaxyShootingStars == 0 {
		return
	}
	for streak := 0; streak < 3; streak++ {
		cycle := math.Mod(s.galaxyTime*.22+float64(streak)*.37, 1)
		if cycle > .26 {
			continue
		}
		progress := cycle / .26
		startX := .08 + hashUnit(streak+801)*.65
		startY := .08 + hashUnit(streak+901)*.32
		for sample := 0; sample < 10; sample++ {
			lag := float64(sample) * .018
			t := math.Max(0, progress-lag)
			strength := (1 - float64(sample)/10) * (1 - smoothstep((progress-.78)/.22))
			x, y := startX+t*.22, startY+t*.17
			splatMaximum(out, width, height, x*float64(width-1), y*float64(height-1),
				math.Max(.8, float64(width)*.004), math.Max(.8, float64(height)*.008),
				210*strength*intensity)
		}
	}
}

func (s *solver) rasterGalaxyParticles(width, height int, intensity float64) []byte {
	return s.rasterGalaxyLayers(width, height, intensity, intensity)
}

// rasterGalaxyLayers keeps the simulated particle field separate from ambient
// raster-only scenery. During gathering, deep-field stars and nebulae can fade
// before the shared particles converge, avoiding the appearance that half of
// the galaxy was left behind in a different buffer.
func (s *solver) rasterGalaxyLayers(width, height int, particleIntensity, backdropIntensity float64) []byte {
	out := make([]byte, width*height)
	s.rasterGalaxyBackdrop(out, width, height, backdropIntensity)
	baseRadiusX := math.Max(.7, float64(width)/float64(s.nx)*.3)
	baseRadiusY := math.Max(.7, float64(height)/float64(s.ny)*.3)
	// Additive broad splats form continuous arm haze below the crisp stars.
	{
		accumulation := make([]float64, width*height)
		for index, p := range s.p {
			star := s.galaxy[index]
			if star.role == galaxyHalo || star.role == galaxyBackground {
				continue
			}
			cx, cy := p.x*float64(width-1), (1-p.y)*float64(height-1)
			radiusX, radiusY := baseRadiusX*2.8, baseRadiusY*2.4
			for y := int(math.Floor(cy - radiusY)); y <= int(math.Ceil(cy+radiusY)); y++ {
				for x := int(math.Floor(cx - radiusX)); x <= int(math.Ceil(cx+radiusX)); x++ {
					if x < 0 || y < 0 || x >= width || y >= height {
						continue
					}
					distance := math.Hypot((float64(x)-cx)/radiusX, (float64(y)-cy)/radiusY)
					if distance < 1 {
						accumulation[y*width+x] += s.particleBrightness(index) * (1 - distance) * .17 * particleIntensity
					}
				}
			}
		}
		for index, value := range accumulation {
			// Screen blending preserves the independently rendered starfield and
			// nebula instead of replacing them with the arm accumulation buffer.
			out[index] = screenBlendByte(out[index], 158*(1-math.Exp(-value/95)))
		}
		// Offset dark lanes lead the luminous density wave. Darkening only the
		// diffuse layers leaves crisp stars visible while carving arm structure.
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				dx := float64(x)/float64(max(1, width-1)) - .5
				dy := (float64(y)/float64(max(1, height-1)) - .47) / .72
				radius := math.Hypot(dx, dy)
				if radius < .08 || radius > .44 {
					continue
				}
				angle := math.Atan2(-dy, dx) - s.galaxyTime*.31
				lane := .5 + .5*math.Cos(2*(angle-logarithmicSpiralAngle(radius)+.13))
				attenuation := 1 - .34*math.Pow(lane, 10)*smoothstep((radius-.08)/.08)
				out[y*width+x] = byte(float64(out[y*width+x]) * attenuation)
			}
		}
	}
	for index, p := range s.p {
		star := s.galaxy[index]
		brightness := s.particleBrightness(index) * particleIntensity
		if star.role == galaxyArm {
			brightness *= .72
		} else if star.role == galaxyDust {
			brightness *= .58
		}
		cx, cy := p.x*float64(width-1), (1-p.y)*float64(height-1)
		splatMaximum(
			out, width, height, cx, cy,
			baseRadiusX*star.splatScale, baseRadiusY*star.splatScale,
			brightness,
		)
	}
	s.rasterShootingStars(out, width, height, backdropIntensity)
	// A soft bright nucleus anchors the spiral when ASCII resolution is low.
	centerX, centerY := .5*float64(width-1), .47*float64(height-1)
	coreRadiusX := math.Max(2, float64(width)*.026)
	coreRadiusY := math.Max(2, float64(height)*.035)
	if s.galaxyEffects&galaxyPulse != 0 {
		pulse := .5 + .5*math.Sin(s.galaxyTime*2.4)
		splatMaximum(out, width, height, centerX, centerY,
			coreRadiusX*(1.5+pulse*.9), coreRadiusY*(1.5+pulse*.9),
			(48+52*pulse)*particleIntensity)
	}
	// Multi-scale bulge: a broad old-star envelope under a hot compact core.
	splatMaximum(out, width, height, centerX, centerY, coreRadiusX*3.3, coreRadiusY*2.8, 48*particleIntensity)
	splatMaximum(out, width, height, centerX, centerY, coreRadiusX*2.1, coreRadiusY*2.0, 105*particleIntensity)
	splatMaximum(out, width, height, centerX, centerY, coreRadiusX, coreRadiusY, 255*particleIntensity)
	return out
}

func (s *solver) rasterGalaxy(width, height int) ([]byte, []byte) {
	return s.rasterGalaxyParticles(width, height, 1), make([]byte, width*height)
}

// rasterLogoParticles draws only the particles that belonged to the struck
// logo. It deliberately excludes galaxy backdrop, arm haze, nucleus, shooting
// stars, and every other ambient galaxy module.
func (s *solver) rasterLogoParticles(width, height int, intensity float64) []byte {
	out := make([]byte, width*height)
	radiusX := math.Max(.7, float64(width)/float64(s.nx)*.34)
	radiusY := math.Max(.7, float64(height)/float64(s.ny)*.34)
	for index, particle := range s.p {
		brightness := (175 + 80*hashUnit(index+1701)) * intensity
		splatMaximum(out, width, height,
			particle.x*float64(width-1), (1-particle.y)*float64(height-1),
			radiusX, radiusY, brightness)
	}
	return out
}

func bezierPoint(start, control, end point, progress float64) point {
	inverse := 1 - progress
	return point{
		x: inverse*inverse*start.x + 2*inverse*progress*control.x + progress*progress*end.x,
		y: inverse*inverse*start.y + 2*inverse*progress*control.y + progress*progress*end.y,
	}
}

func splatLabel(out []byte, width, height int, cx, cy, radiusX, radiusY float64, label byte) {
	for y := int(math.Floor(cy - radiusY)); y <= int(math.Ceil(cy+radiusY)); y++ {
		for x := int(math.Floor(cx - radiusX)); x <= int(math.Ceil(cx+radiusX)); x++ {
			if x < 0 || y < 0 || x >= width || y >= height {
				continue
			}
			if math.Hypot((float64(x)-cx)/radiusX, (float64(y)-cy)/radiusY) <= 1 && label > out[y*width+x] {
				out[y*width+x] = label
			}
		}
	}
}

func randomRecurringCometDelayFrames() int {
	return recurringCometDelayMinFrames + rand.IntN(recurringCometDelayMaxFrames-recurringCometDelayMinFrames+1)
}

func (s *solver) randomizeRecurringCometPath() {
	impact := point{x: .5, y: .5}
	if minX, minY, maxX, maxY, ok := s.logoBounds(); ok && s.targetWidth > 1 && s.targetHeight > 1 {
		impact = point{
			x: float64(minX+maxX) / 2 / float64(s.targetWidth-1),
			y: 1 - float64(minY+maxY)/2/float64(s.targetHeight-1),
		}
	}
	angle := rand.Float64() * 2 * math.Pi
	direction := point{x: math.Cos(angle), y: math.Sin(angle)}
	start := point{x: impact.x + direction.x*.9, y: impact.y + direction.y*.9}
	bend := (.08 + rand.Float64()*.12)
	if rand.Float64() < .5 {
		bend = -bend
	}
	s.recurringCometPath = cometPath{
		start: start,
		control: point{
			x: (start.x+impact.x)/2 - direction.y*bend,
			y: (start.y+impact.y)/2 + direction.x*bend,
		},
		impact: impact,
	}
}

func (s *solver) rasterComet(width, height int, progress float64) ([]byte, []byte, []byte) {
	return s.rasterCometOverPath(width, height, progress, s.rasterGalaxyParticles(width, height, 1), nil, initialCometPath)
}

func (s *solver) rasterRecurringComet(width, height int, progress float64, base, baseAccent []byte) ([]byte, []byte, []byte) {
	return s.rasterCometOverPath(width, height, progress, base, baseAccent, s.recurringCometPath)
}

// rasterCometOverPath preserves the scene beneath the projectile. In recurring
// cycles that scene is the still-rotating solid logo rather than a galaxy
// particle approximation, so the collision visibly happens to the object.
func (s *solver) rasterCometOverPath(width, height int, progress float64, base, baseAccent []byte, path cometPath) ([]byte, []byte, []byte) {
	out := make([]byte, width*height)
	copy(out, base)
	accent := make([]byte, width*height)
	copy(accent, baseAccent)
	const trailSamples = 22
	for sample := trailSamples - 1; sample >= 0; sample-- {
		lag := float64(sample) / float64(trailSamples-1) * .42
		t := math.Max(0, progress-lag)
		position := bezierPoint(path.start, path.control, path.impact, t)
		strength := math.Exp(-float64(sample) * .16)
		if progress-lag <= 0 && sample > 0 {
			strength *= math.Max(0, 1-float64(sample)/5)
		}
		cx := position.x * float64(width-1)
		cy := (1 - position.y) * float64(height-1)
		radiusX, radiusY := math.Max(1.2, float64(width)*.012), math.Max(1.5, float64(height)*.025)
		splatMaximum(out, width, height, cx, cy, radiusX, radiusY, 245*strength)
		splatLabel(accent, width, height, cx, cy, radiusX, radiusY, 128)
	}
	head := bezierPoint(path.start, path.control, path.impact, progress)
	headX, headY := head.x*float64(width-1), (1-head.y)*float64(height-1)
	headRadiusX, headRadiusY := math.Max(2, float64(width)*.018), math.Max(2, float64(height)*.035)
	splatMaximum(out, width, height, headX, headY, headRadiusX, headRadiusY, 255)
	splatLabel(accent, width, height, headX, headY, headRadiusX, headRadiusY, 255)
	return out, make([]byte, width*height), accent
}

func (s *solver) beginImpactAt(center point) {
	if s.galaxyImpactApplied {
		return
	}
	s.galaxyImpactApplied = true
	for index := range s.p {
		p := &s.p[index]
		dx, dy := p.x-center.x, p.y-center.y
		distance := math.Max(.025, math.Hypot(dx, dy))
		impulse := .16 + .62*math.Exp(-distance/.20)
		p.vx = p.vx*.18 + dx/distance*impulse - dy/distance*.13
		p.vy = p.vy*.18 + dy/distance*impulse + dx/distance*.13
	}
}

func (s *solver) rasterGalaxyImpactHold(width, height int) ([]byte, []byte) {
	return rasterImpactHold(width, height, s.rasterGalaxyParticles(width, height, 1), initialCometPath.impact)
}

func rasterImpactHold(width, height int, base []byte, center point) ([]byte, []byte) {
	out := make([]byte, width*height)
	copy(out, base)
	// Freeze the collision for one overexposed frame before releasing its energy.
	for index, value := range out {
		out[index] = byte(min(255, int(value)+int(value)/3))
	}
	centerX, centerY := center.x*float64(width-1), (1-center.y)*float64(height-1)
	splatMaximum(out, width, height, centerX, centerY, math.Max(3, float64(width)*.075), math.Max(3, float64(height)*.11), 255)
	return out, make([]byte, width*height)
}

func (s *solver) stepImpactAt(dt, progress float64, center point) {
	s.beginImpactAt(center)
	waveRadius := .30 * smoothstep(progress)
	for index := range s.p {
		p := &s.p[index]
		dx, dy := p.x-center.x, p.y-center.y
		distance := math.Max(.001, math.Hypot(dx, dy))
		// The narrow moving force band physically displaces particles as the
		// visible shock ring reaches them, rather than painting a passive overlay.
		bandDistance := (distance - waveRadius) / .035
		waveForce := math.Exp(-bandDistance*bandDistance) * .34
		p.vx += dx / distance * waveForce * dt
		p.vy += dy / distance * waveForce * dt
		p.x = math.Max(0, math.Min(1, p.x+p.vx*dt))
		p.y = math.Max(0, math.Min(1, p.y+p.vy*dt))
		p.vx *= .965
		p.vy *= .965
	}
	s.sequence++
}

func (s *solver) stepGalaxyImpact(dt, progress float64) {
	s.stepImpactAt(dt, progress, initialCometPath.impact)
}

func (s *solver) stepLogoImpact(dt, progress float64) {
	s.stepImpactAt(dt, progress, s.recurringCometPath.impact)
}

func rasterImpactOver(width, height int, progress float64, out []byte, center point) ([]byte, []byte) {
	centerX, centerY := center.x*float64(width-1), (1-center.y)*float64(height-1)
	flash := 1 - smoothstep(progress/.28)
	splatMaximum(out, width, height, centerX, centerY, math.Max(2, float64(width)*.07)*flash, math.Max(2, float64(height)*.10)*flash, 255*flash)
	if progress < .92 {
		ringRadiusX := float64(width) * (.025 + .25*smoothstep(progress))
		ringRadiusY := float64(height) * (.035 + .20*smoothstep(progress))
		thickness := math.Max(1, float64(width)*.008)
		fade := 1 - smoothstep((progress-.58)/.34)
		for y := max(0, int(centerY-ringRadiusY-thickness)); y <= min(height-1, int(centerY+ringRadiusY+thickness)); y++ {
			for x := max(0, int(centerX-ringRadiusX-thickness)); x <= min(width-1, int(centerX+ringRadiusX+thickness)); x++ {
				distance := math.Hypot((float64(x)-centerX)/ringRadiusX, (float64(y)-centerY)/ringRadiusY)
				value := 220 * fade * math.Max(0, 1-math.Abs(distance-1)*ringRadiusX/thickness)
				if byte(value) > out[y*width+x] {
					out[y*width+x] = byte(value)
				}
			}
		}
	}
	return out, make([]byte, width*height)
}

func (s *solver) rasterGalaxyImpact(width, height int, progress float64) ([]byte, []byte) {
	return rasterImpactOver(width, height, progress, s.rasterGalaxyParticles(width, height, 1), initialCometPath.impact)
}

func (s *solver) rasterLogoImpact(width, height int, progress float64) ([]byte, []byte) {
	return rasterImpactOver(width, height, progress, s.rasterLogoParticles(width, height, 1), s.recurringCometPath.impact)
}

func smoothstep(value float64) float64 {
	value = math.Max(0, math.Min(1, value))
	return value * value * (3 - 2*value)
}

// stepLogoGather transitions from the physical ocean into a directed particle
// flow. Early motion retains momentum and adds curl; late motion becomes a
// critically damped approach to the exact SVG-derived destinations.
func (s *solver) stepLogoGather(dt, progress float64) {
	s.stepLogoGatherToward(dt, progress, s.targets)
}

func (s *solver) rotatingLogoTargets(angle float64) []point {
	if len(s.targets) != len(s.p) || s.targetWidth < 2 || s.targetHeight < 2 {
		return nil
	}
	minX, minY, maxX, maxY, ok := s.logoBounds()
	if !ok {
		return nil
	}
	centerX, centerY := float64(minX+maxX)/2, float64(minY+maxY)/2
	halfDepth := s.logoDepth() / 2
	cameraDistance := math.Max(float64(s.targetWidth), float64(s.targetHeight)) * 3.5
	projectedTargets := make([]point, len(s.targets))
	for index, target := range s.targets {
		local := point{
			x: target.x*float64(s.targetWidth) - .5 - centerX,
			y: (1-target.y)*float64(s.targetHeight) - .5 - centerY,
		}
		z := (hashUnit(index+1201)*2 - 1) * halfDepth
		projected, _ := projectLogoPoint(local, z, angle, cameraDistance)
		projectedTargets[index] = point{
			x: math.Max(0, math.Min(1, (centerX+projected.x)/float64(s.targetWidth-1))),
			y: math.Max(0, math.Min(1, 1-(centerY+projected.y)/float64(s.targetHeight-1))),
		}
	}
	return projectedTargets
}

func (s *solver) advanceLogoRotation(dt float64) {
	// Ease through edge-on views but linger when the mark faces toward or away
	// from the viewer. The speed remains smooth and never reaches zero.
	sine := math.Sin(s.logoAngle)
	// A 30% minimum creates a readable front/back pause; sin² keeps the dwell
	// broad instead of slowing for only a single perfectly aligned frame.
	speed := .30 + 1.20*sine*sine
	s.logoAngle = math.Mod(s.logoAngle+logoRotationRadiansPerSecond*speed*dt, 2*math.Pi)
}

func (s *solver) stepRotatingLogoGather(dt, progress, angle float64) {
	s.stepLogoGatherToward(dt, progress, s.rotatingLogoTargets(angle))
}

func (s *solver) stepLogoGatherToward(dt, progress float64, targets []point) {
	if len(targets) != len(s.p) {
		s.sequence++
		return
	}
	eased := smoothstep(progress)
	centerX, centerY := .5, .5
	if len(s.gatherOrigins) != len(s.p) {
		s.gatherOrigins = make([]point, len(s.p))
		for index, p := range s.p {
			s.gatherOrigins[index] = point{x: p.x, y: p.y}
		}
	}
	for index := range s.p {
		p := &s.p[index]
		target := targets[index]
		if index%9 == 0 && len(s.gatherOrigins) == len(s.p) {
			origin := s.gatherOrigins[index]
			overshoot := smoothstep((progress-.50)/.20) * (1 - smoothstep((progress-.86)/.10)) * .10
			target.x += (target.x - origin.x) * overshoot
			target.y += (target.y - origin.y) * overshoot
		}
		dx, dy := target.x-p.x, target.y-p.y
		cx, cy := p.x-centerX, p.y-centerY
		radius := math.Max(.04, math.Hypot(cx, cy))
		swirl := math.Sin(math.Pi*math.Min(1, progress*1.25)) * (1 - eased) * 1.15
		desiredX := dx*(1.2+7.8*eased) - cy/radius*swirl
		desiredY := dy*(1.2+7.8*eased) + cx/radius*swirl
		response := math.Min(1, dt*(3+15*eased))
		p.vx += (desiredX - p.vx) * response
		p.vy += (desiredY - p.vy) * response
		p.x += p.vx * dt
		p.y += p.vy * dt
		if progress > .9 {
			snap := smoothstep((progress-.9)/.1) * .22
			p.x += (target.x - p.x) * snap
			p.y += (target.y - p.y) * snap
			p.vx *= 1 - snap
			p.vy *= 1 - snap
		}
		p.x = math.Max(0, math.Min(1, p.x))
		p.y = math.Max(0, math.Min(1, p.y))
	}
	s.sequence++
}

func (s *solver) rasterLogo(width, height int) ([]byte, []byte) {
	out := make([]byte, width*height)
	if width == s.targetWidth && height == s.targetHeight {
		copy(out, s.targetAlpha)
	}
	return out, make([]byte, width*height)
}

// logoBounds returns the visible target dimensions used to choose an extrusion
// depth. The depth follows the mark rather than the viewport, so resizes do not
// make the object look thicker merely because more blank canvas is available.
func (s *solver) logoBounds() (int, int, int, int, bool) {
	minX, minY, maxX, maxY := s.targetWidth, s.targetHeight, -1, -1
	for index, alpha := range s.targetAlpha {
		if alpha <= 4 {
			continue
		}
		x, y := index%s.targetWidth, index/s.targetWidth
		minX, minY = min(minX, x), min(minY, y)
		maxX, maxY = max(maxX, x), max(maxY, y)
	}
	return minX, minY, maxX, maxY, maxX >= minX && maxY >= minY
}

// projectLogoPoint rotates a local 3-D point around the logo's vertical center
// axis and applies restrained perspective. Returning coordinates relative to
// the center makes the pivot exact and independently testable.
func projectLogoPoint(local point, z, angle, cameraDistance float64) (point, float64) {
	cosine, sine := math.Cos(angle), math.Sin(angle)
	rotatedX := local.x*cosine + z*sine
	rotatedZ := -local.x*sine + z*cosine
	denominator := math.Max(cameraDistance*.25, cameraDistance-rotatedZ)
	scale := cameraDistance / denominator
	return point{x: rotatedX * scale, y: local.y * scale}, rotatedZ
}

func (s *solver) logoDepth() float64 {
	minX, _, maxX, _, ok := s.logoBounds()
	if !ok {
		return 1
	}
	return math.Max(1.5, float64(maxX-minX+1)*.09)
}

func paintProjectedLogoSample(out, surfaces []byte, depths []float64, width, height int, cx, cy, depth, brightness float64, surface byte) {
	const radius = 1.05
	for y := int(math.Floor(cy - radius)); y <= int(math.Ceil(cy+radius)); y++ {
		for x := int(math.Floor(cx - radius)); x <= int(math.Ceil(cx+radius)); x++ {
			if x < 0 || y < 0 || x >= width || y >= height {
				continue
			}
			coverage := math.Max(0, 1-math.Hypot(float64(x)-cx, float64(y)-cy)/radius)
			if coverage == 0 {
				continue
			}
			index := y*width + x
			value := byte(math.Max(0, math.Min(255, brightness*coverage)))
			if depth > depths[index]+.08 {
				depths[index], out[index], surfaces[index] = depth, value, surface
			} else if math.Abs(depth-depths[index]) <= .08 && value > out[index] {
				out[index], surfaces[index] = value, surface
			}
		}
	}
}

// rasterRotatingLogo turns the alpha target into a shallow solid: the two logo
// faces and the boundary walls are projected with a z-buffer and independent
// lighting. This produces a readable front view, visible thickness edge-on,
// and a shaded reverse face without requiring a terminal-side 3-D renderer.
func (s *solver) rasterRotatingLogo(width, height int, angle float64) ([]byte, []byte, []byte) {
	out := make([]byte, width*height)
	land := make([]byte, width*height)
	surfaces := make([]byte, width*height)
	if width != s.targetWidth || height != s.targetHeight || len(s.targetAlpha) != width*height {
		return out, land, surfaces
	}
	minX, minY, maxX, maxY, ok := s.logoBounds()
	if !ok {
		return out, land, surfaces
	}
	depths := make([]float64, width*height)
	for index := range depths {
		depths[index] = math.Inf(-1)
	}
	centerX, centerY := float64(minX+maxX)/2, float64(minY+maxY)/2
	halfDepth := s.logoDepth() / 2
	cameraDistance := math.Max(float64(width), float64(height)) * 3.5
	cosine, sine := math.Cos(angle), math.Sin(angle)
	paint := func(x, y int, z, shade float64, alpha, surface byte) {
		local := point{x: float64(x) - centerX, y: float64(y) - centerY}
		projected, projectedDepth := projectLogoPoint(local, z, angle, cameraDistance)
		paintProjectedLogoSample(out, surfaces, depths, width, height,
			centerX+projected.x, centerY+projected.y, projectedDepth, float64(alpha)*shade, surface)
	}
	// Far face first is not required by the z-buffer, but makes equal-depth
	// grazing angles deterministic.
	for _, face := range []struct {
		z, shade float64
		surface  byte
	}{
		{-halfDepth, .52 + .48*math.Max(0, -cosine), logoSurfaceBack},
		{halfDepth, .52 + .48*math.Max(0, cosine), logoSurfaceFront},
	} {
		for index, alpha := range s.targetAlpha {
			if alpha <= 4 {
				continue
			}
			paint(index%width, index/width, face.z, face.shade, alpha, face.surface)
		}
	}
	depthSamples := max(2, int(math.Ceil(halfDepth*2))+1)
	visible := func(x, y int) bool {
		return x >= 0 && y >= 0 && x < width && y < height && s.targetAlpha[y*width+x] > 4
	}
	for index, alpha := range s.targetAlpha {
		if alpha <= 4 {
			continue
		}
		x, y := index%width, index/width
		normalX := 0.0
		boundary := false
		if !visible(x-1, y) {
			normalX--
			boundary = true
		}
		if !visible(x+1, y) {
			normalX++
			boundary = true
		}
		if !visible(x, y-1) || !visible(x, y+1) {
			boundary = true
		}
		if !boundary {
			continue
		}
		sideShade := .38 + .50*math.Max(0, -normalX*sine)
		for sample := 0; sample < depthSamples; sample++ {
			z := -halfDepth + 2*halfDepth*float64(sample)/float64(depthSamples-1)
			paint(x, y, z, sideShade, alpha, logoSurfaceEdge)
		}
	}
	return out, land, surfaces
}

// Synchronize the fluid markers with the currently displayed solid before a
// submitted message applies its one-shot liquidation force. Deterministic depth
// samples distribute markers through the extrusion and avoid a flat-logo jump.
func (s *solver) placeParticlesOnRotatingLogo(angle float64) {
	projectedTargets := s.rotatingLogoTargets(angle)
	if len(projectedTargets) != len(s.p) {
		return
	}
	for index, target := range projectedTargets {
		s.p[index].x, s.p[index].y = target.x, target.y
		s.p[index].vx, s.p[index].vy = 0, 0
	}
}

func (s *solver) rasterGather(width, height int, progress float64) ([]byte, []byte) {
	return s.rasterGatherWithTarget(width, height, progress, s.targetAlpha)
}

func (s *solver) rasterRotatingGather(width, height int, progress, angle float64) ([]byte, []byte, []byte) {
	projected, _, surfaces := s.rasterRotatingLogo(width, height, angle)
	out, land := s.rasterGatherWithTarget(width, height, progress, projected)
	filterEmergingLogoSurfaces(out, projected, surfaces, smoothstep((progress-.58)/.42))
	return out, land, surfaces
}

// rasterRotatingLogoRegather rebuilds from logo debris only. Reusing the
// initial galaxy gather here would reintroduce its starfield, nebula, nucleus,
// and arm haze after every recurring collision.
func (s *solver) rasterRotatingLogoRegather(width, height int, progress, angle float64) ([]byte, []byte, []byte) {
	projected, _, surfaces := s.rasterRotatingLogo(width, height, angle)
	particleFade := 1 - smoothstep((progress-.78)/.22)
	out := s.rasterLogoParticles(width, height, particleFade)
	blend := smoothstep((progress - .58) / .42)
	blendLogoTarget(out, projected, blend)
	filterEmergingLogoSurfaces(out, projected, surfaces, blend)
	return out, make([]byte, width*height), surfaces
}

func filterEmergingLogoSurfaces(out, projected, surfaces []byte, blend float64) {
	for index := range surfaces {
		// A surface vocabulary starts only when that projected target sample has
		// actually overtaken the fading particles at this pixel.
		projectedValue := byte(float64(projected[index]) * blend)
		if projectedValue == 0 || projectedValue < out[index] {
			surfaces[index] = 0
		}
	}
}

func (s *solver) rasterGatherWithTarget(width, height int, progress float64, targetAlpha []byte) ([]byte, []byte) {
	// Reuse the selected galaxy renderer so visual modules remain continuous at
	// the handoff. The rotating target takes over only after particles converge.
	particleFade := 1 - smoothstep((progress-.78)/.22)
	// Continuous nebula and transient streaks are raster scenery, so remove them
	// early. Deep-field stars are persistent particles and gather with the disk.
	backdropFade := 1 - smoothstep(progress/.32)
	out := s.rasterGalaxyLayers(width, height, particleFade, backdropFade)
	blendLogoTarget(out, targetAlpha, smoothstep((progress-.58)/.42))
	return out, make([]byte, width*height)
}

func blendLogoTarget(out, targetAlpha []byte, blend float64) {
	if len(targetAlpha) != len(out) {
		return
	}
	for index, target := range targetAlpha {
		value := byte(float64(target) * blend)
		if value > out[index] {
			out[index] = value
		}
	}
}
