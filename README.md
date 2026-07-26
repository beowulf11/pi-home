# Custom Pi intro

Global Pi extension that:

- converts `source.png` into responsive ASCII art with the Go-based `ascii-image-converter` program;
- centers the art using the current terminal width and height and renders it in `#F28954`;
- regenerates and caches the art when the terminal size or source image changes;
- chooses a hard-coded intro profile from the Pi session's working directory;
- plays the Praktik one-shot, dynamically rasterized entrance animation under `~/code/praktik`: SVG components move at fractional-pixel positions inside a clipped viewport, and every complete frame is reconverted to ASCII;
- plays a background Go PIC/FLIP fluid intro everywhere else, with the procedural wave retained as an immediate/missing-binary fallback;
- shows a minimal update list directly above the editor only when updates exist, then removes it as soon as input or agent work begins;
- replaces Pi's built-in version/package update notices to avoid duplicates.

## Intro profiles

Profiles are currently defined in `intro-config.ts`. The first configured root containing `ctx.cwd` wins; exact roots and all descendants match without accidentally matching similarly prefixed sibling paths.

| Profile | Working directory | Animation |
|---|---|---|
| `praktik` | `~/code/praktik` or any descendant | `praktik-entry` |
| `default` | Everything else | `default-wave` |

The default profile uses the FLIP pipeline described below. `default-wave-animation.ts` remains an isolated fallback for startup before the first fluid frame and when the local executable is absent. New hard-coded project profiles can be added to `INTRO_PROFILES` before a generic configuration format is introduced.

## Default FLIP pipeline

The default intro deliberately separates three layers:

1. `fluid/` is a minimal Go 2-D PIC/FLIP solver and occupancy rasterizer. It uses particles, staggered MAC-grid transfers, gravity/advection, pressure projection, a 95% FLIP / 5% PIC grid-to-particle update, solid tank boundaries, and grayscale particle rasterization. Source attribution to Matthias Müller's MIT-licensed Ten Minute Physics FLIP work is in `fluid/solver.go`, with the license text in `fluid/THIRD_PARTY_LICENSES.md`.
2. `fluid-transport.ts` owns the persistent child process and line protocol. Pi sends `resize <pixelWidth> <pixelHeight>` and `stop`; Go returns `frame <sequence> <width> <height> <base64 grayscale bytes>`. Only the newest valid complete frame is retained. stderr is drained away from the TUI.
3. `raster-to-ascii.ts` is pure TypeScript area resampling/density conversion with terminal-cell aspect compensation and optional deterministic temporal hysteresis.

The child is started lazily by the default header's first animated render, never while the extension factory loads. Its initial resize determines the stable physics grid. Later terminal resizes change only raster output dimensions, preserving normalized particle positions and velocity. The existing 60 fps / `ENTRANCE_END_FRAME` lifecycle owns the process: animation completion, user input, agent start, header disposal, and session shutdown stop it idempotently. The last buffered frame remains available for the static header. Praktik profile behavior is unchanged.

Build, test, and preview locally:

```sh
cd ~/.pi/agent/extensions/custom-intro
npm run build:fluid
npm test
npm run test:fluid
npm run preview:fluid          # optional width/height: ... -- 64 20
```

The platform executable is generated at ignored `bin/fluid-intro`; it is intentionally not committed. Node 24 runs the TypeScript tests/scripts directly.

## Artwork

The source image is:

```text
~/.pi/agent/extensions/custom-intro/source.png
```

The canonical vector source is tracked alongside it as `logo.svg`, copied from:

```text
/Users/beowulf/Documents/Projects/Praktik/Graphics/SYMBOL/COLOR/Praktik-Symbol-Color.svg
```

`source.png` is a 512×512 transparent raster generated from that SVG for `ascii-image-converter`. Replacing `source.png` is detected on the next render. Until it exists, `ascii-art.txt` is used as a fallback.

## Converter

Installed globally through Homebrew:

```sh
brew install TheZoraiz/ascii-image-converter/ascii-image-converter
brew install imagemagick
```

The stored source is trimmed to its visible bounds to avoid wasting output space on transparent padding. The extension uses `ascii-image-converter --complex` at normal sizes and falls back to a simpler character map on very small terminals. Logo caps scale with the terminal: 48×20 normally, 72×30 from 140×52 terminals, and 96×40 from 220×70 terminals (the square source and character aspect ratio can make the effective width smaller when the row cap wins). Vertical margins adapt to terminal height: one row below 32 terminal rows, two below 44, and four on larger screens. Only the rows used by an actual update notice are reserved, so small screens no longer shrink the logo or add empty space for an absent notice. It first sizes by terminal width and, only when necessary, recalculates by terminal height.

Animations run once only for an empty session. For the Praktik profile, `svg-ascii-animation.ts` splits the canonical compound SVG path into its five components, rasterizes those masks once at 4× the target character resolution, moves them with bilinear fractional-pixel sampling inside one fixed clipped rectangle, and converts every full raster back through the same simple or complex density map. Frames are cached by terminal/logo size. This avoids moving rigid ASCII characters between whole cells; edge glyphs evolve as the underlying SVG crosses character boundaries. The old character translator remains only as a fallback if SVG rasterization is unavailable.

The animation immediately completes if the user types, submits input, or agent work begins. Timing is controlled in `logo-animation.ts` by `ENTRANCE_FPS`, `ENTRANCE_INTERVAL_MS`, `DOT_BUILD_END_FRAME`, `ARROW_ENTRY_START_FRAME`, and `ARROW_ENTRY_END_FRAME`.

Render every animation frame as `.txt`, `.svg`, and `.png`, plus a PNG contact sheet, with:

```sh
cd ~/.pi/agent/extensions/custom-intro
npm run render-frames -- --height 20
npm run render-frames -- --height 18 --simple
npm run render-frames -- --height 12 --out /tmp/praktik-frames
```

The default output is `frames/h<height>-complex/` (or `-simple`) and is intentionally ignored by Git. This uses the exact same SVG-to-ASCII frame generator as Pi, so frame previews cannot drift from the extension.

Run `/reload` after changing extension code. The global `quietStartup` setting hides Pi's built-in skills/extensions listing so the intro stays clean. Set `PI_OFFLINE=1` to disable all update checks. Existing `PI_SKIP_VERSION_CHECK=1` and `PI_SKIP_PACKAGE_UPDATE_CHECK=1` settings disable their respective custom checks.

## Git branches

- `main`: static baseline.
- `animation-test`: random character and pulse experiments.
- `logo-entry-animation`: SVG-derived, component-aware entrance animation.
