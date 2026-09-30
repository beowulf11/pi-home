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
	liquidationMaxFrames          = 420
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
	s.gatherWaypoints = nil
	s.gatherDelays = nil
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

// stepLiquidation returns true once every particle has fallen through the open
// bottom. A finite upper bound remains as a safety net for unexpected states.
func (s *solver) stepLiquidation(dt float64) bool {
	s.beginLiquidation()
	s.stepPhysics(dt, false, .96)
	s.liquidationFrames++
	if len(s.p) == 0 {
		return true
	}
	if s.liquidationFrames >= liquidationMaxFrames {
		s.p = nil
		return true
	}
	return false
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

func (s *solver) randomizeImpact(path cometPath, minimumStrength, strengthRange, maximumSpin float64) {
	dx, dy := path.impact.x-path.control.x, path.impact.y-path.control.y
	length := math.Max(.0001, math.Hypot(dx, dy))
	s.impactDirection = point{x: dx / length, y: dy / length}
	s.impactStrength = minimumStrength + rand.Float64()*strengthRange
	s.impactSpin = (rand.Float64()*2 - 1) * maximumSpin
	s.impactStyle = rand.IntN(5)
	s.impactDuration = 14 + rand.IntN(17)
	s.impactScale = .72 + rand.Float64()*.62
	s.impactAspect = .58 + rand.Float64()*1.05
}

func (s *solver) currentImpactFrames() int {
	if s.impactDuration == 0 {
		return experimentImpactFrames
	}
	return s.impactDuration
}

func (s *solver) randomizeInitialCometPath() {
	// Every intro gets a different collision, not merely a different wait before
	// the same collision. Keep the hit near the galaxy's dense body so the fluid
	// response remains legible at small terminal sizes.
	impact := point{x: .38 + rand.Float64()*.24, y: .41 + rand.Float64()*.20}
	entryAngle := math.Pi * (.08 + rand.Float64()*.84)
	direction := point{x: math.Cos(entryAngle), y: math.Sin(entryAngle)}
	start := point{x: impact.x + direction.x*1.05, y: impact.y + direction.y*1.05}
	bend := .08 + rand.Float64()*.18
	if rand.Float64() < .5 {
		bend = -bend
	}
	s.activeCometPath = cometPath{
		start: start,
		control: point{
			x: (start.x+impact.x)/2 - direction.y*bend,
			y: (start.y+impact.y)/2 + direction.x*bend,
		},
		impact: impact,
	}
	s.randomizeImpact(s.activeCometPath, .78, .55, .24)
}

func (s *solver) initialPath() cometPath {
	if s.activeCometPath == (cometPath{}) {
		return initialCometPath
	}
	return s.activeCometPath
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
	s.randomizeImpact(s.recurringCometPath, .72, .62, .28)
}

func (s *solver) rasterComet(width, height int, progress float64) ([]byte, []byte, []byte) {
	return s.rasterCometOverPath(width, height, progress, s.rasterGalaxyParticles(width, height, 1), nil, s.initialPath())
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
	direction := s.impactDirection
	if direction == (point{}) {
		direction = point{x: 1}
	}
	normal := point{x: -direction.y, y: direction.x}
	for index := range s.p {
		p := &s.p[index]
		dx, dy := p.x-center.x, p.y-center.y
		distance := math.Max(.025, math.Hypot(dx, dy))
		rx, ry := dx/distance, dy/distance
		strength := s.impactStrength
		if strength == 0 {
			strength = 1
		}
		impulse := strength * (.16 + .62*math.Exp(-distance/.20))
		vx, vy := rx, ry
		switch s.impactStyle {
		case 1: // directional punch: most debris continues with the meteor
			vx, vy = rx*.30+direction.x*.92, ry*.30+direction.y*.92
		case 2: // vortex burst: collision energy rolls around the contact point
			spin := 1.0
			if s.impactSpin < 0 {
				spin = -1
			}
			vx, vy = rx*.48-ry*.88*spin, ry*.48+rx*.88*spin
		case 3: // split/shear: the logo tears into two opposing sheets
			side := 1.0
			if dx*normal.x+dy*normal.y < 0 {
				side = -1
			}
			vx, vy = direction.x*.28+normal.x*side, direction.y*.28+normal.y*side
		case 4: // crater: a narrow forward cone breaks out into a radial rim
			alignment := math.Max(0, rx*direction.x+ry*direction.y)
			vx, vy = rx*(.55+.55*alignment)+direction.x*.45, ry*(.55+.55*alignment)+direction.y*.45
		}
		spin := s.impactSpin
		p.vx = p.vx*.18 + vx*impulse - ry*spin
		p.vy = p.vy*.18 + vy*impulse + rx*spin
	}
}

func (s *solver) rasterGalaxyImpactHold(width, height int) ([]byte, []byte) {
	return s.rasterImpactHold(width, height, s.rasterGalaxyParticles(width, height, 1), s.initialPath().impact)
}

func (s *solver) rasterImpactHold(width, height int, base []byte, center point) ([]byte, []byte) {
	out := make([]byte, width*height)
	copy(out, base)
	// Freeze the collision for one overexposed frame before releasing its energy.
	for index, value := range out {
		out[index] = byte(min(255, int(value)+int(value)/3))
	}
	centerX, centerY := center.x*float64(width-1), (1-center.y)*float64(height-1)
	scale, aspect := s.impactScale, s.impactAspect
	if scale == 0 {
		scale = 1
	}
	if aspect == 0 {
		aspect = 1
	}
	splatMaximum(out, width, height, centerX, centerY,
		math.Max(3, float64(width)*.075*scale*aspect),
		math.Max(3, float64(height)*.11*scale/aspect), 255)
	return out, make([]byte, width*height)
}

func (s *solver) stepImpactAt(dt, progress float64, center point) {
	s.beginImpactAt(center)
	scale := s.impactScale
	if scale == 0 {
		scale = 1
	}
	aspect := s.impactAspect
	if aspect == 0 {
		aspect = 1
	}
	direction := s.impactDirection
	if direction == (point{}) {
		direction = point{x: 1}
	}
	normal := point{x: -direction.y, y: direction.x}
	waveRadius := .30 * scale * smoothstep(progress)
	for index := range s.p {
		p := &s.p[index]
		dx, dy := p.x-center.x, p.y-center.y
		along, across := dx*direction.x+dy*direction.y, dx*normal.x+dy*normal.y
		distance := math.Max(.001, math.Hypot(along/aspect, across*aspect))
		bandDistance := (distance - waveRadius) / (.028 + .014*scale)
		waveForce := math.Exp(-bandDistance*bandDistance) * (.24 + .18*scale)
		rx, ry := dx/math.Max(.001, math.Hypot(dx, dy)), dy/math.Max(.001, math.Hypot(dx, dy))
		fx, fy := rx, ry
		switch s.impactStyle {
		case 1:
			fx, fy = direction.x, direction.y
		case 2:
			fx, fy = -ry, rx
			if s.impactSpin < 0 {
				fx, fy = -fx, -fy
			}
		case 3:
			side := 1.0
			if across < 0 {
				side = -1
			}
			fx, fy = normal.x*side, normal.y*side
		case 4:
			pulse := .55 + .45*math.Sin(progress*math.Pi*3)
			fx, fy = rx*pulse+direction.x*(1-pulse), ry*pulse+direction.y*(1-pulse)
		}
		p.vx += fx * waveForce * dt
		p.vy += fy * waveForce * dt
		p.x = math.Max(0, math.Min(1, p.x+p.vx*dt))
		p.y = math.Max(0, math.Min(1, p.y+p.vy*dt))
		p.vx *= .965
		p.vy *= .965
	}
	s.sequence++
}

func (s *solver) stepGalaxyImpact(dt, progress float64) {
	s.stepImpactAt(dt, progress, s.initialPath().impact)
}

func (s *solver) stepLogoImpact(dt, progress float64) {
	s.stepImpactAt(dt, progress, s.recurringCometPath.impact)
}

func (s *solver) rasterImpactOver(width, height int, progress float64, out []byte, center point) ([]byte, []byte) {
	centerX, centerY := center.x*float64(width-1), (1-center.y)*float64(height-1)
	scale, aspect := s.impactScale, s.impactAspect
	if scale == 0 {
		scale = 1
	}
	if aspect == 0 {
		aspect = 1
	}
	direction := s.impactDirection
	if direction == (point{}) {
		direction = point{x: 1}
	}
	normal := point{x: -direction.y, y: direction.x}
	flash := 1 - smoothstep(progress/(.20+.14*scale))
	flashX := math.Max(2, float64(width)*.065*scale*aspect) * flash
	flashY := math.Max(2, float64(height)*.085*scale/aspect) * flash
	splatMaximum(out, width, height, centerX, centerY, flashX, flashY, 255*flash)
	paintRing := func(ringProgress, brightness, offset float64) {
		if ringProgress < 0 || ringProgress >= .94 {
			return
		}
		ringRadius := (.025 + .27*smoothstep(ringProgress)) * scale
		cx := center.x + direction.x*offset*smoothstep(ringProgress)
		cy := center.y + direction.y*offset*smoothstep(ringProgress)
		thickness := .007 + .004*scale
		fade := 1 - smoothstep((ringProgress-.54)/.40)
		for y := 0; y < height; y++ {
			worldY := 1 - float64(y)/float64(max(1, height-1))
			for x := 0; x < width; x++ {
				worldX := float64(x) / float64(max(1, width-1))
				dx, dy := worldX-cx, worldY-cy
				along := (dx*direction.x + dy*direction.y) / aspect
				across := (dx*normal.x + dy*normal.y) * aspect
				distance := math.Hypot(along, across)
				value := brightness * fade * math.Max(0, 1-math.Abs(distance-ringRadius)/thickness)
				if byte(value) > out[y*width+x] {
					out[y*width+x] = byte(value)
				}
			}
		}
	}
	offset := 0.0
	if s.impactStyle == 1 || s.impactStyle == 4 {
		offset = .09
	}
	paintRing(progress, 220, offset)
	if s.impactStyle == 2 || s.impactStyle == 3 || s.impactStyle == 4 {
		paintRing(progress-.18, 155, -offset*.45)
	}
	return out, make([]byte, width*height)
}

func (s *solver) rasterGalaxyImpact(width, height int, progress float64) ([]byte, []byte) {
	return s.rasterImpactOver(width, height, progress, s.rasterGalaxyParticles(width, height, 1), s.initialPath().impact)
}

func (s *solver) rasterLogoImpact(width, height int, progress float64) ([]byte, []byte) {
	return s.rasterImpactOver(width, height, progress, s.rasterLogoParticles(width, height, 1), s.recurringCometPath.impact)
}

func smoothstep(value float64) float64 {
	value = math.Max(0, math.Min(1, value))
	return value * value * (3 - 2*value)
}

// prepareDynamicGather rebuilds particle-to-logo correspondence from the
// positions produced by this particular collision. A coarse randomized spatial
// ordering keeps matching O(P log P), while chunk delays and shared waypoints
// make groups of debris take distinct routes instead of replaying one morph.
func (s *solver) prepareDynamicGather() {
	if len(s.targets) != len(s.p) || len(s.p) == 0 {
		return
	}
	const chunksX, chunksY = 8, 6
	angle := rand.Float64() * 2 * math.Pi
	cosine, sine := math.Cos(angle), math.Sin(angle)
	reverse := rand.IntN(2) == 0
	spatialKey := func(p point) (int, float64) {
		dx, dy := p.x-.5, p.y-.5
		x := math.Max(0, math.Min(.999999, .5+dx*cosine-dy*sine))
		y := math.Max(0, math.Min(.999999, .5+dx*sine+dy*cosine))
		cellX, cellY := int(x*chunksX), int(y*chunksY)
		if (cellY%2 == 1) != reverse {
			cellX = chunksX - 1 - cellX
		}
		return cellY*chunksX + cellX, y + x*.01
	}
	particleOrder := make([]int, len(s.p))
	targetOrder := make([]int, len(s.targets))
	for index := range particleOrder {
		particleOrder[index], targetOrder[index] = index, index
	}
	sort.Slice(particleOrder, func(i, j int) bool {
		a := s.p[particleOrder[i]]
		b := s.p[particleOrder[j]]
		ka, la := spatialKey(point{x: a.x, y: a.y})
		kb, lb := spatialKey(point{x: b.x, y: b.y})
		return ka < kb || (ka == kb && la < lb)
	})
	sort.Slice(targetOrder, func(i, j int) bool {
		ka, la := spatialKey(s.targets[targetOrder[i]])
		kb, lb := spatialKey(s.targets[targetOrder[j]])
		return ka < kb || (ka == kb && la < lb)
	})

	availableTargets := append([]point(nil), s.targets...)
	s.gatherOrigins = make([]point, len(s.p))
	s.gatherWaypoints = make([]point, len(s.p))
	s.gatherDelays = make([]float64, len(s.p))
	s.gatherCenter = point{x: .32 + rand.Float64()*.36, y: .32 + rand.Float64()*.36}
	s.gatherStyle = rand.IntN(5)
	curlRanges := [5][2]float64{{1.25, 2.15}, {.12, .42}, {.28, .72}, {.85, 1.45}, {.24, .65}}
	curlRange := curlRanges[s.gatherStyle]
	s.gatherCurl = curlRange[0] + rand.Float64()*(curlRange[1]-curlRange[0])
	if rand.IntN(2) == 0 {
		s.gatherCurl = -s.gatherCurl
	}
	flowAngle := rand.Float64() * 2 * math.Pi
	flowX, flowY := math.Cos(flowAngle), math.Sin(flowAngle)
	chunkDelay := make([]float64, chunksX*chunksY)
	chunkBend := make([]float64, chunksX*chunksY)
	for chunk := range chunkDelay {
		chunkX, chunkY := chunk%chunksX, chunk/chunksX
		cx := (float64(chunkX)+.5)/chunksX - .5
		cy := (float64(chunkY)+.5)/chunksY - .5
		jitter := rand.Float64() * .045
		switch s.gatherStyle {
		case 0: // vortex: chunks peel off in a loose, irregular spiral
			chunkDelay[chunk] = rand.Float64() * .14
		case 1: // sweep: a diagonal front assembles the mark from one side
			projection := (cx*flowX + cy*flowY + .72) / 1.44
			chunkDelay[chunk] = math.Max(0, math.Min(.30, projection*.30+jitter))
		case 2: // split: outside fragments fold inward in two opposing wings
			chunkDelay[chunk] = (1-math.Min(1, math.Abs(cx)*2))*.23 + jitter
		case 3: // collapse/bloom: debris implodes before expanding into the mark
			chunkDelay[chunk] = rand.Float64() * .09
		case 4: // wave: neighboring chunks arrive in visibly separated bands
			chunkDelay[chunk] = (.5+.5*math.Sin(float64(chunkX)*1.35+float64(chunkY)*.82+flowAngle))*.26 + jitter
		}
		chunkBend[chunk] = (rand.Float64()*2 - 1) * .24
	}
	for rank, particleIndex := range particleOrder {
		target := availableTargets[targetOrder[rank]]
		s.targets[particleIndex] = target
		origin := point{x: s.p[particleIndex].x, y: s.p[particleIndex].y}
		s.gatherOrigins[particleIndex] = origin
		chunkX := clamp(int(target.x*chunksX), 0, chunksX-1)
		chunkY := clamp(int(target.y*chunksY), 0, chunksY-1)
		chunk := chunkY*chunksX + chunkX
		chunkCenter := point{x: (float64(chunkX) + .5) / chunksX, y: (float64(chunkY) + .5) / chunksY}
		dx, dy := chunkCenter.x-origin.x, chunkCenter.y-origin.y
		waypoint := point{
			x: (origin.x+chunkCenter.x)/2 - dy*chunkBend[chunk],
			y: (origin.y+chunkCenter.y)/2 + dx*chunkBend[chunk],
		}
		switch s.gatherStyle {
		case 1:
			waypoint.x += -flowX*.13 - flowY*.22
			waypoint.y += -flowY*.13 + flowX*.22
		case 2:
			side := 1.0
			if chunkCenter.x < .5 {
				side = -1
			}
			waypoint.x += side * .28
			waypoint.y += (chunkCenter.y - .5) * .16
		case 3:
			waypoint.x = s.gatherCenter.x + (chunkCenter.x-.5)*.08
			waypoint.y = s.gatherCenter.y + (chunkCenter.y-.5)*.08
		case 4:
			wave := math.Sin(float64(chunkX)*1.35 + float64(chunkY)*.82 + flowAngle)
			waypoint.x += -dy * wave * .32
			waypoint.y += dx * wave * .32
		}
		s.gatherWaypoints[particleIndex] = waypoint
		s.gatherDelays[particleIndex] = chunkDelay[chunk]
	}
}

// stepLogoGather transitions from the physical debris into a directed particle
// flow. Matching and routes are rebuilt at the start of every gather.
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
	if len(s.gatherOrigins) != len(s.p) || len(s.gatherWaypoints) != len(s.p) {
		s.prepareDynamicGather()
	}
	for index := range s.p {
		p := &s.p[index]
		finalTarget := targets[index]
		delay := 0.0
		if len(s.gatherDelays) == len(s.p) {
			delay = s.gatherDelays[index]
		}
		localProgress := math.Max(0, math.Min(1, (progress-delay)/(1-delay)))
		eased := smoothstep(localProgress)
		target := finalTarget
		if len(s.gatherOrigins) == len(s.p) && len(s.gatherWaypoints) == len(s.p) {
			// Follow a per-cycle quadratic route through a shared chunk waypoint.
			// The endpoint may rotate, but debris within a chunk travels together.
			target = bezierPoint(s.gatherOrigins[index], s.gatherWaypoints[index], finalTarget, eased)
		}
		if index%9 == 0 {
			overshoot := smoothstep((localProgress-.50)/.20) * (1 - smoothstep((localProgress-.86)/.10)) * .10
			target.x += (finalTarget.x - s.gatherOrigins[index].x) * overshoot
			target.y += (finalTarget.y - s.gatherOrigins[index].y) * overshoot
		}
		dx, dy := target.x-p.x, target.y-p.y
		cx, cy := p.x-s.gatherCenter.x, p.y-s.gatherCenter.y
		radius := math.Max(.04, math.Hypot(cx, cy))
		swirl := math.Sin(math.Pi*math.Min(1, localProgress*1.25)) * (1 - eased) * s.gatherCurl
		desiredX := dx*(1.6+9.2*eased) - cy/radius*swirl
		desiredY := dy*(1.6+9.2*eased) + cx/radius*swirl
		response := math.Min(1, dt*(3+16*eased))
		p.vx += (desiredX - p.vx) * response
		p.vy += (desiredY - p.vy) * response
		p.x += p.vx * dt
		p.y += p.vy * dt
		if localProgress > .88 {
			snap := smoothstep((localProgress-.88)/.12) * .25
			p.x += (finalTarget.x - p.x) * snap
			p.y += (finalTarget.y - p.y) * snap
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
	filterEmergingLogoSurfaces(out, projected, surfaces, smoothstep((progress-.76)/.24))
	return out, land, surfaces
}

// rasterRotatingLogoRegather rebuilds from logo debris only. Reusing the
// initial galaxy gather here would reintroduce its starfield, nebula, nucleus,
// and arm haze after every recurring collision.
func (s *solver) rasterRotatingLogoRegather(width, height int, progress, angle float64) ([]byte, []byte, []byte) {
	projected, _, surfaces := s.rasterRotatingLogo(width, height, angle)
	particleFade := 1 - smoothstep((progress-.88)/.12)
	out := s.rasterLogoParticles(width, height, particleFade)
	blend := smoothstep((progress - .76) / .24)
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
	particleFade := 1 - smoothstep((progress-.88)/.12)
	// Continuous nebula and transient streaks are raster scenery, so remove them
	// early. Deep-field stars are persistent particles and gather with the disk.
	backdropFade := 1 - smoothstep(progress/.32)
	out := s.rasterGalaxyLayers(width, height, particleFade, backdropFade)
	// Keep the real debris visible for most of the morph; the exact target only
	// takes over late enough to guarantee a crisp final frame.
	blendLogoTarget(out, targetAlpha, smoothstep((progress-.76)/.24))
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
