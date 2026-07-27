package main

import (
	"image/png"
	"math"
	"os"
	"sort"
)

const (
	experimentGalaxyFrames        = 180
	experimentCometDelayMinFrames = 180
	experimentCometDelayMaxFrames = 360
	experimentCometFrames         = 48
	experimentImpactHoldFrames    = 1
	experimentImpactFrames        = 18
	experimentGatherFrames        = 210
)

type point struct{ x, y float64 }

type galaxyStyle byte

const (
	galaxyClassic galaxyStyle = iota
	galaxyLiving
)

type galaxyRole byte

const (
	galaxyCore galaxyRole = iota
	galaxyArm
	galaxyKnot
	galaxyHalo
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

// initializeGalaxy repurposes the fluid markers as a deterministic two-arm
// spiral. Styles are deliberately selectable so the experimental treatment can
// evolve without changing the established timed intro.
func (s *solver) initializeGalaxy(style galaxyStyle) {
	if s.experimentInitialized {
		return
	}
	s.experimentInitialized = true
	s.galaxyStyle = style
	s.galaxy = make([]galaxyParticle, len(s.p))
	coreCount := max(1, len(s.p)/6)
	for index := range s.p {
		if style == galaxyClassic {
			randomA := hashUnit(index*3 + 1)
			randomB := hashUnit(index*3 + 2)
			classicCoreCount := max(1, len(s.p)/7)
			var radius, angle float64
			if index < classicCoreCount {
				radius = .085 * math.Sqrt(randomA)
				angle = randomB * 2 * math.Pi
			} else {
				t := float64(index-classicCoreCount) / float64(max(1, len(s.p)-classicCoreCount-1))
				radius = .055 + .39*math.Sqrt(t)
				angle = float64(index%2)*math.Pi + radius*15.5 + (randomA-.5)*(.22+.52*t)
			}
			s.galaxy[index] = galaxyParticle{
				role: galaxyArm, radius: radius, angle: angle,
				baseRadius: radius, baseAngle: angle,
				angularSpeed: .24 + .36*(1-radius/.46),
				brightness:   byte(125 + int(randomB*130)), splatScale: 1,
			}
			continue
		}
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
			if style == galaxyLiving && randomC < .14 {
				star.role = galaxyHalo
				star.baseRadius = .16 + .32*math.Sqrt(randomA)
				star.baseAngle = randomB * 2 * math.Pi
				star.brightness = byte(90 + int(randomC*350))
				star.splatScale = .65
			} else {
				star.role = galaxyArm
				if style == galaxyLiving && index%13 == 0 {
					star.role = galaxyKnot
					star.brightness = byte(220 + int(randomC*35))
					star.splatScale = 1.35
				}
				star.baseRadius = .055 + .40*math.Sqrt(t)
				arm := float64(index%2) * math.Pi
				jitter := (randomA - .5) * (.18 + .45*t)
				star.baseAngle = arm + star.baseRadius*15.2 + jitter
			}
		}
		star.radius = star.baseRadius
		star.angle = star.baseAngle
		star.angularSpeed = .24 + .36*(1-star.baseRadius/.48)
		star.epicyclePhase = randomC * 2 * math.Pi
		star.epicycleRate = .9 + randomA*1.25
		s.galaxy[index] = star
	}
	s.placeGalaxyParticles(0)
}

func (s *solver) placeGalaxyParticles(dt float64) {
	s.galaxyTime += dt
	tilt := 0.0
	if s.galaxyStyle == galaxyLiving {
		tilt = .11
	}
	for index := range s.p {
		star := &s.galaxy[index]
		oldX, oldY := s.p[index].x, s.p[index].y
		var radius, angle float64
		if s.galaxyStyle == galaxyLiving {
			breath := .008 * math.Sin(s.galaxyTime*1.15+star.epicyclePhase)
			epicycle := .018 * math.Sin(star.epicyclePhase+s.galaxyTime*star.epicycleRate)
			radius = star.baseRadius * (1 + breath)
			angle = star.baseAngle + s.galaxyTime*.29 + epicycle
			if star.role == galaxyHalo {
				angle = star.baseAngle + s.galaxyTime*(.12+.08*hashUnit(index+91))
			}
		} else {
			star.angle += star.angularSpeed * dt
			angle = star.angle
			radius = star.radius + .008*math.Sin(star.angle*3+float64(index%17))
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
	if s.galaxyStyle == galaxyClassic {
		return float64(star.brightness) * (.76 + .24*math.Sin(float64(s.sequence)*.11+float64(index%31)*1.7))
	}
	amplitude := .04
	if star.role == galaxyKnot || star.role == galaxyHalo {
		amplitude = .18
	}
	return float64(star.brightness) * (1 - amplitude + amplitude*math.Sin(star.twinklePhase+s.galaxyTime*star.twinkleRate))
}

func (s *solver) rasterGalaxyParticles(width, height int, intensity float64) []byte {
	out := make([]byte, width*height)
	baseRadiusX := math.Max(.7, float64(width)/float64(s.nx)*.3)
	baseRadiusY := math.Max(.7, float64(height)/float64(s.ny)*.3)
	if s.galaxyStyle == galaxyLiving {
		// Additive broad splats form continuous arm haze below the crisp stars.
		accumulation := make([]float64, width*height)
		for index, p := range s.p {
			star := s.galaxy[index]
			if star.role == galaxyHalo {
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
						accumulation[y*width+x] += s.particleBrightness(index) * (1 - distance) * .17 * intensity
					}
				}
			}
		}
		for index, value := range accumulation {
			out[index] = byte(150 * (1 - math.Exp(-value/95)))
		}
	}
	for index, p := range s.p {
		star := s.galaxy[index]
		brightness := s.particleBrightness(index) * intensity
		if s.galaxyStyle == galaxyLiving && star.role == galaxyArm {
			brightness *= .72
		}
		cx, cy := p.x*float64(width-1), (1-p.y)*float64(height-1)
		splatMaximum(
			out, width, height, cx, cy,
			baseRadiusX*star.splatScale, baseRadiusY*star.splatScale,
			brightness,
		)
	}
	// A soft bright nucleus anchors the spiral when ASCII resolution is low.
	centerX, centerY := .5*float64(width-1), .47*float64(height-1)
	coreRadiusX := math.Max(2, float64(width)*.026)
	coreRadiusY := math.Max(2, float64(height)*.035)
	if s.galaxyStyle == galaxyLiving {
		splatMaximum(out, width, height, centerX, centerY, coreRadiusX*2.2, coreRadiusY*2.2, 92*intensity)
	}
	splatMaximum(out, width, height, centerX, centerY, coreRadiusX, coreRadiusY, 255*intensity)
	return out
}

func (s *solver) rasterGalaxy(width, height int) ([]byte, []byte) {
	return s.rasterGalaxyParticles(width, height, 1), make([]byte, width*height)
}

func bezierPoint(start, control, end point, progress float64) point {
	inverse := 1 - progress
	return point{
		x: inverse*inverse*start.x + 2*inverse*progress*control.x + progress*progress*end.x,
		y: inverse*inverse*start.y + 2*inverse*progress*control.y + progress*progress*end.y,
	}
}

func (s *solver) rasterComet(width, height int, progress float64) ([]byte, []byte) {
	out := s.rasterGalaxyParticles(width, height, 1)
	start, control, impact := point{x: -.08, y: .91}, point{x: .18, y: .72}, point{x: .5, y: .53}
	const trailSamples = 22
	for sample := trailSamples - 1; sample >= 0; sample-- {
		lag := float64(sample) / float64(trailSamples-1) * .42
		t := math.Max(0, progress-lag)
		position := bezierPoint(start, control, impact, t)
		strength := math.Exp(-float64(sample) * .16)
		if progress-lag <= 0 && sample > 0 {
			strength *= math.Max(0, 1-float64(sample)/5)
		}
		cx := position.x * float64(width-1)
		cy := (1 - position.y) * float64(height-1)
		splatMaximum(out, width, height, cx, cy, math.Max(1.2, float64(width)*.012), math.Max(1.5, float64(height)*.025), 245*strength)
	}
	head := bezierPoint(start, control, impact, progress)
	splatMaximum(out, width, height, head.x*float64(width-1), (1-head.y)*float64(height-1), math.Max(2, float64(width)*.018), math.Max(2, float64(height)*.035), 255)
	return out, make([]byte, width*height)
}

func (s *solver) beginGalaxyImpact() {
	if s.galaxyImpactApplied {
		return
	}
	s.galaxyImpactApplied = true
	for index := range s.p {
		p := &s.p[index]
		dx, dy := p.x-.5, p.y-.53
		distance := math.Max(.025, math.Hypot(dx, dy))
		impulse := .16 + .62*math.Exp(-distance/.20)
		p.vx = p.vx*.18 + dx/distance*impulse - dy/distance*.13
		p.vy = p.vy*.18 + dy/distance*impulse + dx/distance*.13
	}
}

func (s *solver) rasterGalaxyImpactHold(width, height int) ([]byte, []byte) {
	out := s.rasterGalaxyParticles(width, height, 1)
	// Freeze the collision for one overexposed frame before releasing its energy.
	for index, value := range out {
		out[index] = byte(min(255, int(value)+int(value)/3))
	}
	centerX, centerY := .5*float64(width-1), .47*float64(height-1)
	splatMaximum(out, width, height, centerX, centerY, math.Max(3, float64(width)*.075), math.Max(3, float64(height)*.11), 255)
	return out, make([]byte, width*height)
}

func (s *solver) stepGalaxyImpact(dt, progress float64) {
	s.beginGalaxyImpact()
	waveRadius := .30 * smoothstep(progress)
	for index := range s.p {
		p := &s.p[index]
		dx, dy := p.x-.5, p.y-.53
		distance := math.Max(.001, math.Hypot(dx, dy))
		// The narrow moving force band physically displaces stars as the visible
		// shock ring reaches them, rather than painting a passive overlay.
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

func (s *solver) rasterGalaxyImpact(width, height int, progress float64) ([]byte, []byte) {
	out := s.rasterGalaxyParticles(width, height, 1)
	centerX, centerY := .5*float64(width-1), .47*float64(height-1)
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

func smoothstep(value float64) float64 {
	value = math.Max(0, math.Min(1, value))
	return value * value * (3 - 2*value)
}

// stepLogoGather transitions from the physical ocean into a directed particle
// flow. Early motion retains momentum and adds curl; late motion becomes a
// critically damped approach to the exact SVG-derived destinations.
func (s *solver) stepLogoGather(dt, progress float64) {
	if len(s.targets) != len(s.p) {
		s.sequence++
		return
	}
	eased := smoothstep(progress)
	centerX, centerY := .5, .5
	if s.galaxyStyle == galaxyLiving && len(s.gatherOrigins) != len(s.p) {
		s.gatherOrigins = make([]point, len(s.p))
		for index, p := range s.p {
			s.gatherOrigins[index] = point{x: p.x, y: p.y}
		}
	}
	for index := range s.p {
		p := &s.p[index]
		target := s.targets[index]
		if s.galaxyStyle == galaxyLiving && index%9 == 0 && len(s.gatherOrigins) == len(s.p) {
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

func (s *solver) rasterGather(width, height int, progress float64) ([]byte, []byte) {
	if s.galaxyStyle == galaxyClassic {
		return s.rasterClassicGather(width, height, progress)
	}
	// Reuse the selected galaxy renderer so visual modules remain continuous at
	// the handoff. The canonical target takes over only after particles converge.
	particleFade := 1 - smoothstep((progress-.78)/.22)
	out := s.rasterGalaxyParticles(width, height, particleFade)
	blend := smoothstep((progress - .58) / .42)
	if width == s.targetWidth && height == s.targetHeight {
		for index, target := range s.targetAlpha {
			value := byte(float64(target) * blend)
			if value > out[index] {
				out[index] = value
			}
		}
	}
	return out, make([]byte, width*height)
}

func (s *solver) rasterClassicGather(width, height int, progress float64) ([]byte, []byte) {
	out := make([]byte, width*height)
	eased := smoothstep(progress)
	particleFade := 1 - smoothstep((progress-.82)/.18)
	baseRadiusX := math.Max(.7, float64(width)/float64(s.nx)*.3)
	baseRadiusY := math.Max(.7, float64(height)/float64(s.ny)*.3)
	radiusScale := 1 + .18*eased
	radiusX, radiusY := baseRadiusX*radiusScale, baseRadiusY*radiusScale
	for index, p := range s.p {
		star := s.galaxy[index]
		twinkle := .76 + .24*math.Sin(float64(s.sequence)*.11+float64(index%31)*1.7)
		starBrightness := float64(star.brightness) * twinkle
		brightness := (starBrightness*(1-eased) + 255*eased) * particleFade
		cx, cy := p.x*float64(width-1), (1-p.y)*float64(height-1)
		splatMaximum(out, width, height, cx, cy, radiusX, radiusY, brightness)
	}
	nucleusFade := 1 - smoothstep(progress/.28)
	centerX, centerY := .5*float64(width-1), .47*float64(height-1)
	coreRadiusX := math.Max(2, float64(width)*.026)
	coreRadiusY := math.Max(2, float64(height)*.035)
	splatMaximum(out, width, height, centerX, centerY, coreRadiusX, coreRadiusY, 255*nucleusFade)
	blend := smoothstep((progress - .58) / .42)
	if width == s.targetWidth && height == s.targetHeight {
		for index, target := range s.targetAlpha {
			value := byte(float64(target) * blend)
			if value > out[index] {
				out[index] = value
			}
		}
	}
	return out, make([]byte, width*height)
}
