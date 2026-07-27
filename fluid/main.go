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
	logoPath := flag.String("logo", "", "PNG target used by fluid-logo-gather")
	flag.Parse()
	var logo *logoSource
	if *mode == "fluid-logo-gather" && *logoPath != "" {
		logo, _ = loadLogoSource(*logoPath)
	}

	commands := make(chan command, 4)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			parts := strings.Fields(scanner.Text())
			if len(parts) == 1 && parts[0] == "stop" {
				commands <- command{kind: "stop"}
				return
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
	for {
		select {
		case c := <-commands:
			if c.kind == "stop" {
				return
			}
			width, height = c.width, c.height
			settledDirty = true
			if s == nil {
				s = newSolver(width, height)
			}
			if logo != nil {
				s.initializeGalaxy()
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
			var pixels, land []byte
			if *mode != "fluid-logo-gather" || logo == nil {
				s.step(1.0 / 60)
				pixels, land = s.raster(width, height)
			} else if s.sequence < experimentGalaxyFrames {
				s.stepGalaxy(1.0 / 60)
				pixels, land = s.rasterGalaxy(width, height)
				phase = "galaxy"
			} else if s.sequence < experimentGalaxyFrames+experimentGatherFrames {
				progress := float64(s.sequence-experimentGalaxyFrames) / float64(experimentGatherFrames-1)
				s.stepLogoGather(1.0/60, progress)
				pixels, land = s.rasterGather(width, height, progress)
				phase = "gather"
			} else {
				if !settledDirty {
					continue
				}
				settledDirty = false
				s.sequence++
				pixels, land = s.rasterLogo(width, height)
				phase = "settled"
			}
			if phase == "" {
				fmt.Printf(
					"frame %d %d %d %s %s\n",
					s.sequence, width, height,
					base64.StdEncoding.EncodeToString(pixels),
					base64.StdEncoding.EncodeToString(land),
				)
			} else {
				fmt.Printf(
					"frame %d %d %d %s %s %s\n",
					s.sequence, width, height,
					base64.StdEncoding.EncodeToString(pixels),
					base64.StdEncoding.EncodeToString(land),
					phase,
				)
			}
		}
	}
}
