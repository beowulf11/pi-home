package main

import (
	"image/png"
	"math"
	"os"
	"sort"
)

const (
	experimentGalaxyFrames = 180
	experimentGatherFrames = 210
)

type point struct{ x, y float64 }

type galaxyParticle struct {
	radius, angle, angularSpeed float64
	brightness                  byte
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
// spiral. Differential rotation and per-star twinkle make it read as a living
// galaxy while preserving a one-to-one set of particles for the later morph.
func (s *solver) initializeGalaxy() {
	if s.experimentInitialized {
		return
	}
	s.experimentInitialized = true
	s.galaxy = make([]galaxyParticle, len(s.p))
	coreCount := max(1, len(s.p)/7)
	for index := range s.p {
		randomA := hashUnit(index*3 + 1)
		randomB := hashUnit(index*3 + 2)
		var radius, angle float64
		if index < coreCount {
			radius = .085 * math.Sqrt(randomA)
			angle = randomB * 2 * math.Pi
		} else {
			t := float64(index-coreCount) / float64(max(1, len(s.p)-coreCount-1))
			radius = .055 + .39*math.Sqrt(t)
			arm := float64(index%2) * math.Pi
			jitter := (randomA - .5) * (.22 + .52*t)
			angle = arm + radius*15.5 + jitter
		}
		s.galaxy[index] = galaxyParticle{
			radius:       radius,
			angle:        angle,
			angularSpeed: .24 + .36*(1-radius/.46),
			brightness:   byte(125 + int(randomB*130)),
		}
	}
	s.placeGalaxyParticles(0)
}

func (s *solver) placeGalaxyParticles(dt float64) {
	for index := range s.p {
		star := &s.galaxy[index]
		star.angle += star.angularSpeed * dt
		oldX, oldY := s.p[index].x, s.p[index].y
		wobble := .008 * math.Sin(star.angle*3+float64(index%17))
		radius := star.radius + wobble
		s.p[index].x = .5 + radius*math.Cos(star.angle)
		s.p[index].y = .53 + radius*.72*math.Sin(star.angle)
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

func (s *solver) rasterGalaxy(width, height int) ([]byte, []byte) {
	out := make([]byte, width*height)
	radiusX := math.Max(.7, float64(width)/float64(s.nx)*.3)
	radiusY := math.Max(.7, float64(height)/float64(s.ny)*.3)
	for index, p := range s.p {
		star := s.galaxy[index]
		twinkle := .76 + .24*math.Sin(float64(s.sequence)*.11+float64(index%31)*1.7)
		brightness := float64(star.brightness) * twinkle
		cx, cy := p.x*float64(width-1), (1-p.y)*float64(height-1)
		for y := int(math.Floor(cy - radiusY)); y <= int(math.Ceil(cy+radiusY)); y++ {
			for x := int(math.Floor(cx - radiusX)); x <= int(math.Ceil(cx+radiusX)); x++ {
				if x < 0 || y < 0 || x >= width || y >= height {
					continue
				}
				distance := math.Hypot((float64(x)-cx)/radiusX, (float64(y)-cy)/radiusY)
				value := byte(brightness * (1 - math.Min(1, distance)))
				if value > out[y*width+x] {
					out[y*width+x] = value
				}
			}
		}
	}
	// A soft bright nucleus anchors the spiral when ASCII resolution is low.
	centerX, centerY := .5*float64(width-1), .47*float64(height-1)
	coreRadiusX := math.Max(2, float64(width)*.026)
	coreRadiusY := math.Max(2, float64(height)*.035)
	for y := int(centerY - coreRadiusY); y <= int(centerY+coreRadiusY); y++ {
		for x := int(centerX - coreRadiusX); x <= int(centerX+coreRadiusX); x++ {
			if x < 0 || y < 0 || x >= width || y >= height {
				continue
			}
			distance := math.Hypot((float64(x)-centerX)/coreRadiusX, (float64(y)-centerY)/coreRadiusY)
			value := byte(255 * math.Max(0, 1-distance))
			if value > out[y*width+x] {
				out[y*width+x] = value
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
	for index := range s.p {
		p := &s.p[index]
		target := s.targets[index]
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
	// Use exactly the same star splats as the galaxy phase. Only their positions
	// and intensity evolve, so the ASCII density map never switches render modes
	// at the phase boundary.
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
		for y := int(math.Floor(cy - radiusY)); y <= int(math.Ceil(cy+radiusY)); y++ {
			for x := int(math.Floor(cx - radiusX)); x <= int(math.Ceil(cx+radiusX)); x++ {
				if x < 0 || y < 0 || x >= width || y >= height {
					continue
				}
				distance := math.Hypot((float64(x)-cx)/radiusX, (float64(y)-cy)/radiusY)
				value := byte(brightness * (1 - math.Min(1, distance)))
				if value > out[y*width+x] {
					out[y*width+x] = value
				}
			}
		}
	}
	// Keep the galaxy nucleus at the boundary and fade it away gradually.
	nucleusFade := 1 - smoothstep(progress/.28)
	centerX, centerY := .5*float64(width-1), .47*float64(height-1)
	coreRadiusX := math.Max(2, float64(width)*.026)
	coreRadiusY := math.Max(2, float64(height)*.035)
	for y := int(centerY - coreRadiusY); y <= int(centerY+coreRadiusY); y++ {
		for x := int(centerX - coreRadiusX); x <= int(centerX+coreRadiusX); x++ {
			if x < 0 || y < 0 || x >= width || y >= height {
				continue
			}
			distance := math.Hypot((float64(x)-centerX)/coreRadiusX, (float64(y)-centerY)/coreRadiusY)
			value := byte(255 * math.Max(0, 1-distance) * nucleusFade)
			if value > out[y*width+x] {
				out[y*width+x] = value
			}
		}
	}
	// Crossfade the converged stars into the canonical raster. At progress 1
	// this output is byte-identical to rasterLogo, preventing a final-frame jump.
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
