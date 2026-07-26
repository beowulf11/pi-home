package main

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type command struct {
	kind          string
	width, height int
}

func main() {
	commands := make(chan command, 4)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			parts := strings.Fields(scanner.Text())
			if len(parts) == 1 && parts[0] == "stop" {
				commands <- command{kind: "stop"}
				return
			}
			if len(parts) == 3 && parts[0] == "resize" {
				w, e1 := strconv.Atoi(parts[1])
				h, e2 := strconv.Atoi(parts[2])
				if e1 == nil && e2 == nil && w > 0 && h > 0 {
					commands <- command{kind: "resize", width: w, height: h}
				}
			}
		}
		commands <- command{kind: "stop"}
	}()
	ticker := time.NewTicker(time.Second / 60)
	defer ticker.Stop()
	var s *solver
	width, height := 0, 0
	for {
		select {
		case c := <-commands:
			if c.kind == "stop" {
				return
			}
			width, height = c.width, c.height
			if s == nil {
				s = newSolver(width, height)
			}
		case <-ticker.C:
			if s == nil {
				continue
			}
			s.step(1.0 / 60)
			pixels, land := s.raster(width, height)
			fmt.Printf(
				"frame %d %d %d %s %s\n",
				s.sequence,
				width,
				height,
				base64.StdEncoding.EncodeToString(pixels),
				base64.StdEncoding.EncodeToString(land),
			)
		}
	}
}
