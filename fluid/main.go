package main

import (
	"bufio"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type command struct {
	kind                               string
	width, height, logoWidth, logoRows int
}

func main() {
	mode := flag.String("mode", "default", "animation mode")
	logoPath := flag.String("logo", "", "PNG target used by galaxy-to-logo modes")
	galaxyStyleName := flag.String("galaxy-style", "", "galaxy visual module: classic or living")
	transitionEffect := flag.String("transition-effect", "", "input transition module: direct or comet")
	galaxyEffectsName := flag.String("galaxy-effects", "", "comma-separated galaxy overlays: nebula, starfield, shooting-stars, pulse")
	flag.Parse()
	if *galaxyStyleName == "" {
		if *mode == "galaxy-logo-on-input" {
			*galaxyStyleName = "living"
		} else {
			*galaxyStyleName = "classic"
		}
	}
	if *transitionEffect == "" {
		if *mode == "galaxy-logo-on-input" {
			*transitionEffect = "comet"
		} else {
			*transitionEffect = "direct"
		}
	}
	var logo *logoSource
	if (*mode == "fluid-logo-gather" || *mode == "galaxy-logo-on-input") && *logoPath != "" {
		logo, _ = loadLogoSource(*logoPath)
	}

	commands := make(chan command, 4)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			parts := strings.Fields(scanner.Text())
			if len(parts) == 1 {
				switch parts[0] {
				case "stop":
					commands <- command{kind: "stop"}
					return
				case "transition":
					commands <- command{kind: "transition"}
					continue
				}
			}
			if (len(parts) == 3 || len(parts) == 5) && parts[0] == "resize" {
				w, e1 := strconv.Atoi(parts[1])
				h, e2 := strconv.Atoi(parts[2])
				resize := command{kind: "resize", width: w, height: h}
				if len(parts) == 5 {
					resize.logoWidth, _ = strconv.Atoi(parts[3])
					resize.logoRows, _ = strconv.Atoi(parts[4])
				}
				if e1 == nil && e2 == nil && w > 0 && h > 0 {
					commands <- resize
				}
			}
		}
		commands <- command{kind: "stop"}
	}()
	ticker := time.NewTicker(time.Second / 60)
	defer ticker.Stop()
	var s *solver
	width, height := 0, 0
	settledDirty := true
	experimentState := "galaxy"
	experimentFrame := 0
	liquidated := false
	autoCometDelay := experimentCometDelayMinFrames + int(
		time.Now().UnixNano()%int64(experimentCometDelayMaxFrames-experimentCometDelayMinFrames+1),
	)
	for {
		select {
		case c := <-commands:
			if c.kind == "stop" {
				return
			}
			if c.kind == "transition" {
				if *mode == "galaxy-logo-on-input" && s != nil && !s.liquidationInitialized {
					// A submitted message liquidates whatever the intro currently shows:
					// orbiting galaxy, comet/impact, gathering particles, or settled logo.
					s.beginLiquidation()
					experimentState = "liquidate"
					liquidated = true
					settledDirty = true
				}
				continue
			}
			width, height = c.width, c.height
			settledDirty = true
			if s == nil {
				s = newSolver(width, height)
			}
			if logo != nil {
				style := galaxyClassic
				if *galaxyStyleName == "living" {
					style = galaxyLiving
				}
				s.galaxyEffects = parseGalaxyEffects(*galaxyEffectsName)
				s.initializeGalaxy(style)
				s.setLogoTarget(
					logo.target(width, height, c.logoWidth, c.logoRows*2),
					width,
					height,
				)
			}
		case <-ticker.C:
			if s == nil {
				continue
			}
			phase := ""
			var pixels, land, accent []byte
			if (*mode != "fluid-logo-gather" && *mode != "galaxy-logo-on-input") || logo == nil {
				s.step(1.0 / 60)
				pixels, land = s.raster(width, height)
			} else if *mode == "fluid-logo-gather" {
				if s.sequence < experimentGalaxyFrames {
					s.stepGalaxy(1.0 / 60)
					pixels, land = s.rasterGalaxy(width, height)
					phase = "galaxy"
				} else if s.sequence < experimentGalaxyFrames+experimentGatherFrames {
					progress := float64(s.sequence-experimentGalaxyFrames) / float64(experimentGatherFrames-1)
					s.stepLogoGather(1.0/60, progress)
					pixels, land = s.rasterGather(width, height, progress)
					phase = "gather"
				} else {
					experimentState = "settled"
				}
			} else {
				switch experimentState {
				case "galaxy":
					s.stepGalaxy(1.0 / 60)
					pixels, land = s.rasterGalaxy(width, height)
					phase = "galaxy"
					experimentFrame++
					if *transitionEffect == "comet" && experimentFrame >= autoCometDelay {
						experimentState, experimentFrame = "comet", 0
					}
				case "liquidate":
					if s.stepLiquidation(1.0 / 60) {
						experimentState = "settled"
					} else {
						pixels, land = s.rasterLiquid(width, height)
						phase = "liquidate"
					}
				case "comet":
					progress := float64(experimentFrame) / float64(experimentCometFrames-1)
					s.stepGalaxy(1.0 / 60)
					pixels, land, accent = s.rasterComet(width, height, progress)
					phase = "comet"
					experimentFrame++
					if experimentFrame >= experimentCometFrames {
						experimentState, experimentFrame = "impact-hold", 0
					}
				case "impact-hold":
					pixels, land = s.rasterGalaxyImpactHold(width, height)
					phase = "impact"
					experimentFrame++
					if experimentFrame >= experimentImpactHoldFrames {
						experimentState, experimentFrame = "impact", 0
					}
				case "impact":
					progress := float64(experimentFrame) / float64(experimentImpactFrames-1)
					s.stepGalaxyImpact(1.0/60, progress)
					pixels, land = s.rasterGalaxyImpact(width, height, progress)
					phase = "impact"
					experimentFrame++
					if experimentFrame >= experimentImpactFrames {
						experimentState, experimentFrame = "gather", 0
					}
				case "gather":
					progress := float64(experimentFrame) / float64(experimentGatherFrames-1)
					s.stepLogoGather(1.0/60, progress)
					pixels, land = s.rasterGather(width, height, progress)
					phase = "gather"
					experimentFrame++
					if experimentFrame >= experimentGatherFrames {
						experimentState = "settled"
					}
				}
			}
			if experimentState == "settled" && phase == "" {
				if !settledDirty {
					continue
				}
				settledDirty = false
				s.sequence++
				if liquidated {
					pixels, land = s.rasterLiquid(width, height)
				} else {
					pixels, land = s.rasterLogo(width, height)
				}
				phase = "settled"
			}
			if len(accent) != width*height {
				accent = make([]byte, width*height)
			}
			if phase == "" {
				fmt.Printf(
					"frame %d %d %d %s %s %s\n",
					s.sequence, width, height,
					base64.StdEncoding.EncodeToString(pixels),
					base64.StdEncoding.EncodeToString(land),
					base64.StdEncoding.EncodeToString(accent),
				)
			} else {
				fmt.Printf(
					"frame %d %d %d %s %s %s %s\n",
					s.sequence, width, height,
					base64.StdEncoding.EncodeToString(pixels),
					base64.StdEncoding.EncodeToString(land),
					base64.StdEncoding.EncodeToString(accent),
					phase,
				)
			}
		}
	}
}
