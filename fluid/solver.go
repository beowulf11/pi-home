// Package main contains a compact 2-D PIC/FLIP liquid solver.
//
// The algorithm is based conceptually on Matthias Müller's “Ten Minute Physics”
// FLIP tutorial/code, © 2022 Matthias Müller, released under the MIT License:
// https://matthias-research.github.io/pages/tenMinutePhysics/index.html
// This implementation is independently written for the custom Pi intro.
package main

import "math"

const (
	wavePeriodFrames = 180 // one incoming swell every three seconds at 60 fps
	wavePulseFrames  = 90  // broad 1.5 second push keeps waves overlapping
	maxParticleSpeed = 2.25
	waterSurface     = .31
	beachStart       = .68
	beachTop         = .36
)

type particle struct{ x, y, vx, vy float64 }

type solver struct {
	nx, ny                             int
	p                                  []particle
	u, v, oldU, oldV, weightU, weightV []float64
	sequence                           uint64
	targets                            []point
	targetAlpha                        []byte
	targetWidth, targetHeight          int
	galaxy                             []galaxyParticle
	experimentInitialized              bool
}

func newSolver(pixelWidth, pixelHeight int) *solver {
	nx := clamp(pixelWidth/2, 24, 64)
	ny := clamp(pixelHeight/3, 16, 36)
	s := &solver{nx: nx, ny: ny}
	s.u = make([]float64, (nx+1)*ny)
	s.oldU = make([]float64, len(s.u))
	s.weightU = make([]float64, len(s.u))
	s.v = make([]float64, nx*(ny+1))
	s.oldV = make([]float64, len(s.v))
	s.weightV = make([]float64, len(s.v))
	// A settled ocean spans the tank and meets an invisible rising beach on the
	// right. Four markers per wet cell preserve enough detail for breaking waves.
	for y := 1; y < ny-2; y++ {
		for x := 1; x < nx-1; x++ {
			for _, o := range [][2]float64{{.27, .27}, {.73, .27}, {.27, .73}, {.73, .73}} {
				px := (float64(x) + o[0]) / float64(nx)
				py := (float64(y) + o[1]) / float64(ny)
				if py > s.terrainHeight(px)+.15/float64(ny) && py < waterSurface {
					s.p = append(s.p, particle{x: px, y: py})
				}
			}
		}
	}
	return s
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
func (s *solver) ui(x, y int) int { return y*(s.nx+1) + x }
func (s *solver) vi(x, y int) int { return y*s.nx + x }

func scatter(a, w []float64, width, height int, gx, gy, value float64) {
	x0 := int(math.Floor(gx))
	y0 := int(math.Floor(gy))
	fx := gx - float64(x0)
	fy := gy - float64(y0)
	for dy := 0; dy <= 1; dy++ {
		for dx := 0; dx <= 1; dx++ {
			x, y := x0+dx, y0+dy
			if x < 0 || y < 0 || x >= width || y >= height {
				continue
			}
			q := (1 - math.Abs(float64(dx)-fx)) * (1 - math.Abs(float64(dy)-fy))
			i := y*width + x
			a[i] += value * q
			w[i] += q
		}
	}
}
func sample(a []float64, width, height int, gx, gy float64) float64 {
	gx = math.Max(0, math.Min(float64(width-1), gx))
	gy = math.Max(0, math.Min(float64(height-1), gy))
	x0 := int(gx)
	y0 := int(gy)
	x1 := clamp(x0+1, 0, width-1)
	y1 := clamp(y0+1, 0, height-1)
	fx := gx - float64(x0)
	fy := gy - float64(y0)
	return (a[y0*width+x0]*(1-fx)+a[y0*width+x1]*fx)*(1-fy) + (a[y1*width+x0]*(1-fx)+a[y1*width+x1]*fx)*fy
}
func (s *solver) sampleVelocity(x, y float64, u, v []float64) (float64, float64) {
	return sample(u, s.nx+1, s.ny, x*float64(s.nx), y*float64(s.ny)-.5), sample(v, s.nx, s.ny+1, x*float64(s.nx)-.5, y*float64(s.ny))
}

func (s *solver) terrainHeight(x float64) float64 {
	base := 1.2 / float64(s.ny)
	t := math.Max(0, math.Min(1, (x-beachStart)/(1-beachStart)))
	// Smoothstep avoids a sharp corner where the flat seabed meets the beach.
	t = t * t * (3 - 2*t)
	return base + (beachTop-base)*t
}

func (s *solver) terrainSlope(x float64) float64 {
	t := math.Max(0, math.Min(1, (x-beachStart)/(1-beachStart)))
	return (beachTop - 1.2/float64(s.ny)) * 6 * t * (1 - t) / (1 - beachStart)
}

func (s *solver) solidCell(x, y int) bool {
	if x < 0 || x >= s.nx || y < 0 || y >= s.ny {
		return true
	}
	cx := (float64(x) + .5) / float64(s.nx)
	cy := (float64(y) + .5) / float64(s.ny)
	return cy < s.terrainHeight(cx)
}

// applyWaveMaker supplies energy that numerical damping would otherwise remove.
// A smooth pulse near the left wall periodically pushes the pool rightward and
// slightly upward, producing a repeating wave without creating/removing water.
func (s *solver) applyWaveMaker(dt float64) bool {
	phase := int(s.sequence % wavePeriodFrames)
	if phase >= wavePulseFrames {
		return false
	}
	envelope := math.Sin(math.Pi * (float64(phase) + .5) / wavePulseFrames)
	for i := range s.p {
		p := &s.p[i]
		if p.x >= .28 {
			continue
		}
		influence := 1 - p.x/.28
		p.vx += 3.2 * envelope * influence * dt
		p.vy += .62 * envelope * influence * dt
	}
	return true
}

// separateParticles prevents the marker particles from collapsing into a few
// dense points over an indefinitely running intro. Distances are measured in
// simulation-cell units and a grid keeps neighbor lookup linear.
func (s *solver) separateParticles() {
	const minimumDistance = .44
	buckets := make([][]int, s.nx*s.ny)
	for i, p := range s.p {
		x := clamp(int(p.x*float64(s.nx)), 0, s.nx-1)
		y := clamp(int(p.y*float64(s.ny)), 0, s.ny-1)
		buckets[y*s.nx+x] = append(buckets[y*s.nx+x], i)
	}
	for i := range s.p {
		p := &s.p[i]
		cellX := clamp(int(p.x*float64(s.nx)), 0, s.nx-1)
		cellY := clamp(int(p.y*float64(s.ny)), 0, s.ny-1)
		for by := max(0, cellY-1); by <= min(s.ny-1, cellY+1); by++ {
			for bx := max(0, cellX-1); bx <= min(s.nx-1, cellX+1); bx++ {
				for _, j := range buckets[by*s.nx+bx] {
					if j <= i {
						continue
					}
					q := &s.p[j]
					dx := (q.x - p.x) * float64(s.nx)
					dy := (q.y - p.y) * float64(s.ny)
					distance := math.Hypot(dx, dy)
					if distance >= minimumDistance {
						continue
					}
					overlap := minimumDistance - distance
					if distance < 1e-8 {
						// Stable, deterministic direction for coincident markers.
						angle := float64((i*37+j*17)%360) * math.Pi / 180
						dx, dy, distance = math.Cos(angle), math.Sin(angle), 1
					}
					push := overlap * .5 / distance
					p.x -= dx * push / float64(s.nx)
					p.y -= dy * push / float64(s.ny)
					q.x += dx * push / float64(s.nx)
					q.y += dy * push / float64(s.ny)
				}
			}
		}
	}
	horizontalMargin := 1.2 / float64(s.nx)
	for i := range s.p {
		s.p[i].x = math.Max(horizontalMargin, math.Min(1-horizontalMargin, s.p[i].x))
		floor := s.terrainHeight(s.p[i].x) + .15/float64(s.ny)
		s.p[i].y = math.Max(floor, math.Min(1-1.2/float64(s.ny), s.p[i].y))
	}
}

func (s *solver) step(dt float64) {
	s.applyWaveMaker(dt)
	clear(s.u)
	clear(s.v)
	clear(s.weightU)
	clear(s.weightV)
	for _, p := range s.p {
		scatter(s.u, s.weightU, s.nx+1, s.ny, p.x*float64(s.nx), p.y*float64(s.ny)-.5, p.vx)
		scatter(s.v, s.weightV, s.nx, s.ny+1, p.x*float64(s.nx)-.5, p.y*float64(s.ny), p.vy)
	}
	for i := range s.u {
		if s.weightU[i] > 0 {
			s.u[i] /= s.weightU[i]
		}
	}
	for i := range s.v {
		if s.weightV[i] > 0 {
			s.v[i] /= s.weightV[i]
		}
	}
	// Save the transferred grid before forces; the FLIP delta must include gravity.
	copy(s.oldU, s.u)
	copy(s.oldV, s.v)
	for i := range s.v {
		s.v[i] -= 1.7 * dt
	}
	for y := 0; y < s.ny; y++ {
		s.u[s.ui(0, y)] = 0
		s.u[s.ui(s.nx, y)] = 0
	}
	for x := 0; x < s.nx; x++ {
		s.v[s.vi(x, 0)] = 0
		s.v[s.vi(x, s.ny)] = 0
	}
	// Incompressibility projection on the staggered MAC grid. The rising
	// right-hand seabed is a solid Neumann boundary; air remains zero pressure.
	pressure := make([]float64, s.nx*s.ny)
	next := make([]float64, len(pressure))
	fluid := make([]bool, len(pressure))
	solid := make([]bool, len(pressure))
	for y := 0; y < s.ny; y++ {
		for x := 0; x < s.nx; x++ {
			solid[y*s.nx+x] = s.solidCell(x, y)
		}
	}
	for _, p := range s.p {
		x := clamp(int(p.x*float64(s.nx)), 1, s.nx-2)
		y := clamp(int(p.y*float64(s.ny)), 1, s.ny-2)
		if !solid[y*s.nx+x] {
			fluid[y*s.nx+x] = true
		}
	}
	// No velocity may cross a face touching the beach.
	for y := 0; y < s.ny; y++ {
		for x := 1; x < s.nx; x++ {
			if solid[y*s.nx+x-1] || solid[y*s.nx+x] {
				s.u[s.ui(x, y)] = 0
			}
		}
	}
	for y := 1; y < s.ny; y++ {
		for x := 0; x < s.nx; x++ {
			if solid[(y-1)*s.nx+x] || solid[y*s.nx+x] {
				s.v[s.vi(x, y)] = 0
			}
		}
	}
	for iter := 0; iter < 28; iter++ {
		for y := 1; y < s.ny-1; y++ {
			for x := 1; x < s.nx-1; x++ {
				i := y*s.nx + x
				if !fluid[i] {
					continue
				}
				div := s.u[s.ui(x+1, y)] - s.u[s.ui(x, y)] + s.v[s.vi(x, y+1)] - s.v[s.vi(x, y)]
				sum, count := 0.0, 0.0
				for _, neighbor := range []int{i - 1, i + 1, i - s.nx, i + s.nx} {
					if !solid[neighbor] {
						sum += pressure[neighbor]
						count++
					}
				}
				if count > 0 {
					next[i] = (sum - div) / count
				}
			}
		}
		pressure, next = next, pressure
	}
	// Apply each pressure gradient to its shared MAC face exactly once.
	for y := 0; y < s.ny; y++ {
		for x := 1; x < s.nx; x++ {
			left := y*s.nx + x - 1
			right := left + 1
			if solid[left] || solid[right] {
				s.u[s.ui(x, y)] = 0
			} else if fluid[left] || fluid[right] {
				s.u[s.ui(x, y)] -= pressure[right] - pressure[left]
			}
		}
	}
	for y := 1; y < s.ny; y++ {
		for x := 0; x < s.nx; x++ {
			bottom := (y-1)*s.nx + x
			top := y*s.nx + x
			if solid[bottom] || solid[top] {
				s.v[s.vi(x, y)] = 0
			} else if fluid[bottom] || fluid[top] {
				s.v[s.vi(x, y)] -= pressure[top] - pressure[bottom]
			}
		}
	}
	for y := 0; y < s.ny; y++ {
		s.u[s.ui(0, y)] = 0
		s.u[s.ui(s.nx, y)] = 0
	}
	for x := 0; x < s.nx; x++ {
		s.v[s.vi(x, 0)] = 0
		s.v[s.vi(x, s.ny)] = 0
	}
	// Grid-to-particle: mostly FLIP, with a little PIC damping.
	for i := range s.p {
		p := &s.p[i]
		picU, picV := s.sampleVelocity(p.x, p.y, s.u, s.v)
		oldU, oldV := s.sampleVelocity(p.x, p.y, s.oldU, s.oldV)
		flipU := p.vx + picU - oldU
		flipV := p.vy + picV - oldV
		p.vx = .95*flipU + .05*picU
		p.vy = .95*flipV + .05*picV
		speed := math.Hypot(p.vx, p.vy)
		if speed > maxParticleSpeed {
			p.vx *= maxParticleSpeed / speed
			p.vy *= maxParticleSpeed / speed
		}
		p.x += p.vx * dt
		p.y += p.vy * dt
		margin := 1.2 / float64(s.nx)
		if p.x < margin {
			p.x = margin
			p.vx = math.Abs(p.vx) * .25
		}
		if p.x > 1-margin {
			p.x = 1 - margin
			p.vx = -math.Abs(p.vx) * .25
		}
		floor := s.terrainHeight(p.x) + .15/float64(s.ny)
		if p.y < floor {
			p.y = floor
			slope := s.terrainSlope(p.x)
			normalLength := math.Hypot(slope, 1)
			nx, ny := -slope/normalLength, 1/normalLength
			intoTerrain := p.vx*nx + p.vy*ny
			if intoTerrain < 0 {
				p.vx -= 1.12 * intoTerrain * nx
				p.vy -= 1.12 * intoTerrain * ny
			}
			p.vx *= .88
		}
		topMargin := 1.2 / float64(s.ny)
		if p.y > 1-topMargin {
			p.y = 1 - topMargin
			p.vy = -math.Abs(p.vy) * .15
		}
	}
	// Two inexpensive relaxation passes preserve visible volume over long runs.
	s.separateParticles()
	s.separateParticles()
	s.sequence++
}

func (s *solver) raster(width, height int) ([]byte, []byte) {
	out := make([]byte, width*height)
	land := make([]byte, width*height)
	// Render the otherwise invisible collision terrain as a restrained,
	// deterministic grain. The brighter surface ridge keeps the shoreline
	// readable while the lower-density interior remains visually secondary.
	widthScale := float64(max(1, width-1))
	heightScale := float64(max(1, height-1))
	surfaceBand := 1.5 / heightScale
	for y := 0; y < height; y++ {
		worldY := 1 - float64(y)/heightScale
		for x := 0; x < width; x++ {
			floor := s.terrainHeight(float64(x) / widthScale)
			if worldY > floor {
				continue
			}
			grain := byte(72 + ((x*37 + y*61 + x*y*7) % 54))
			if floor-worldY < surfaceBand {
				grain = 168
			}
			out[y*width+x] = grain
			// The flat tank floor stays visually part of the water silhouette.
			// Only the rising right-hand beach receives the sand material.
			if float64(x)/widthScale > beachStart {
				land[y*width+x] = 255
			}
		}
	}
	// Scale particle splats with the simulation cells, not output pixels. A
	// large terminal therefore shows a continuous body of water instead of
	// spreading a fixed particle count into nearly invisible isolated dots.
	radiusX := math.Max(1.8, float64(width)/float64(s.nx)*.82)
	radiusY := math.Max(1.8, float64(height)/float64(s.ny)*.82)
	for _, p := range s.p {
		cx := p.x * float64(width-1)
		cy := (1 - p.y) * float64(height-1)
		x0, x1 := int(math.Floor(cx-radiusX)), int(math.Ceil(cx+radiusX))
		y0, y1 := int(math.Floor(cy-radiusY)), int(math.Ceil(cy+radiusY))
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				if x < 0 || y < 0 || x >= width || y >= height {
					continue
				}
				dx := (float64(x) - cx) / radiusX
				dy := (float64(y) - cy) / radiusY
				d := math.Hypot(dx, dy)
				value := int(255 * (1 - math.Min(1, d)))
				i := y*width + x
				if value > int(out[i]) {
					out[i] = byte(value)
					land[i] = 0
				}
			}
		}
	}
	return out, land
}
