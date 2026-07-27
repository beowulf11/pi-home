# Custom Pi intro

Global Pi extension that:

- converts `source.png` into responsive ASCII art with the Go-based `ascii-image-converter` program;
- centers the art using the current terminal width and height, rendering the logo/water in `#F28954` and beach land in sandy `#D6B56E`;
- regenerates and caches the art when the terminal size or source image changes;
- chooses a hard-coded intro profile from the Pi session's working directory;
- picks one of several galaxy routes on every displayed intro, with independently composable galaxy, transition, and ambient-effect modules, then smoothly gathers into the responsive Praktik logo;
- keeps `/tmp` and `/private/tmp` as explicit aliases for the latest experimental profile so future candidates can be tested there before promotion;
- retains the Go PIC/FLIP beach-wave implementation as an available variant and immediate/missing-binary fallback;
- shows a minimal update list directly above the editor only when updates exist, then removes it as soon as input or agent work begins;
- replaces Pi's built-in version/package update notices to avoid duplicates.

## Intro profiles

Profiles are currently defined in `intro-config.ts`. The first configured root containing `ctx.cwd` wins; exact roots and all descendants match without accidentally matching similarly prefixed sibling paths.

| Profile | Working directory | Animation |
|---|---|---|
| `experiment` | `/tmp`, `/private/tmp`, or any descendant | `galaxy-logo-on-input` |
| `praktik` | `$HOME/code/praktik` or any descendant | `galaxy-logo-on-input` |
| `default` | Everything else | `galaxy-logo-on-input` |

Every displayed intro picks one preset route and keeps it stable across renders/resizes. The next session can pick another. The built-in routes are:

| Preset | Galaxy | Transition | Combined ambient effects |
|---|---|---|---|
| `classic-drift` | classic | timed comet/impact | starfield |
| `living-nebula` | living | timed comet/impact | nebula + nucleus pulse |
| `comet-trail` | living | timed comet/impact | starfield |
| `meteor-shower` | living | timed comet/impact | starfield + shooting stars |
| `cosmic-storm` | living | timed comet/impact | nebula + starfield + shooting stars + pulse |

Set `PI_GALAXY_VARIANT` to force either a preset or a composable path. For example:

```sh
PI_GALAXY_VARIANT=meteor-shower pi
PI_GALAXY_VARIANT='classic/direct/nebula+starfield' pi
PI_GALAXY_VARIANT='living/comet/nebula+shooting-stars+pulse' pi
```

A custom path is `style/transition/effect+effect`: styles are `classic` and `living`; transitions are `direct` and `comet`; effects are `nebula`, `starfield`, `shooting-stars`, and `pulse`. Invalid paths fall back to the profile's random preset pool. Preset pools can be customized per profile with `galaxyVariants` in `intro-config.ts`.

The original intro sequence remains intact: every built-in preset runs `galaxy → comet → impact → logo` (an explicitly forced custom `direct` route remains available for debugging). A submitted message then liquidates whichever particle state is currently visible in place. Each run creates an invisible randomized force field with a slightly offset center, directional bias, swirl, and strength. At the instant of liquidation it gives every particle a related but spatially different velocity, producing a coherent curved burst instead of uniform movement or unrelated noise. The force is applied only once and is never rendered; afterward only gravity, pressure, collisions, and damping act on the liquid, with no later random forces. Liquidation has its own clean particle-only view and an invisible flat collision floor: it does not render the legacy wave scene's floor grain, side walls, beach slope, or sandy land material. The water falls into the bottom of the viewport, stabilizes, emits one final `settled` frame, and leaves the Go process idle. Liquidation starts only after a submitted message reaches `before_agent_start`; editing text, waiting on the intro, or running a handled command does not trigger it. A terminal resize requests one updated settled frame.

All profiles use the galaxy-to-logo pipeline. `default-wave-animation.ts` remains an isolated fallback for startup before the first generated frame and when the local executable is absent. The retained FLIP implementation is described below. New hard-coded project profiles can be added to `INTRO_PROFILES` before a generic configuration format is introduced.

## Retained FLIP pipeline

The fluid renderer deliberately separates three layers:

1. `fluid/` is a minimal Go 2-D PIC/FLIP solver and occupancy rasterizer. It uses particles, staggered MAC-grid transfers, gravity/advection, pressure projection, a 95% FLIP / 5% PIC grid-to-particle update, solid tank boundaries, and grayscale particle rasterization. Water volume is conserved, two spatial-hash particle-separation passes prevent visual collapse, and particle speed is bounded for long-running stability. The scene now begins as a settled ocean over a flat seabed that rises into an invisible smooth beach on the right. Broad overlapping swells enter from the left every three seconds, shoal against that solid slope, run upward, break, and flow back. The collision floor and beach are rasterized as a restrained deterministic ASCII grain with a brighter surface ridge. The flat submerged floor remains water-colored so the ocean reaches the bottom of the canvas; only the rising right-hand shore receives the sandy land color. Raster splats scale with simulation cells so the same persistent water body remains continuous on large terminals. Source attribution to Matthias Müller's MIT-licensed Ten Minute Physics FLIP work is in `fluid/solver.go`, with the license text in `fluid/THIRD_PARTY_LICENSES.md`.
2. `fluid-transport.ts` owns the persistent child process and line protocol. Pi sends `resize <pixelWidth> <pixelHeight>`, `transition`, and `stop`; Go returns `frame <sequence> <width> <height> <base64 grayscale bytes> <base64 land-mask bytes> <base64 accent-mask bytes>`, optionally followed by an animation phase (`ocean`, `galaxy`, `comet`, `impact`, `gather`, `liquidate`, or `settled`). Only the newest valid complete frame is retained. stderr is drained away from the TUI.
3. `raster-to-ascii.ts` is pure TypeScript area resampling/density conversion with terminal-cell aspect compensation and optional deterministic temporal hysteresis. It independently resamples the material mask so ANSI styling cannot alter geometry or character selection.

The child is started lazily by the animated header's first render, never while the extension factory loads. Its initial resize determines the stable physics grid. Later terminal resizes change only raster output dimensions, preserving normalized particle positions and velocity. The fluid canvas uses the complete terminal width and all available terminal rows except the adaptive top/bottom margins and Pi's editor/footer reservation. At 60 fps it continues for as long as the empty intro is idle, then agent start, header disposal, or session shutdown stops it idempotently. The last buffered frame remains available for the static header. The legacy finite Praktik entrance remains available in the codebase but is no longer assigned to a profile.

Build, test, and preview locally:

```sh
cd ~/code/personal/pi/fancy-intro
npm run build:fluid
npm test
npm run test:fluid
npm run preview:fluid          # optional: ... -- 64 20 galaxy-logo-on-input 180 living comet nebula,starfield,pulse
```

The platform executable is generated at ignored `bin/fluid-intro`; it is intentionally not committed. Node 24 runs the TypeScript tests/scripts directly.

## Artwork

The source image is:

```text
~/code/personal/pi/fancy-intro/source.png
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

Animations run once only for an empty session. The retained `svg-ascii-animation.ts` implementation splits the canonical compound SVG path into its five components, rasterizes those masks once at 4× the target character resolution, moves them with bilinear fractional-pixel sampling inside one fixed clipped rectangle, and converts every full raster back through the same simple or complex density map. Frames are cached by terminal/logo size. This legacy entrance is not currently assigned to a profile.

The galaxy/comet/logo intro runs normally and liquidates only after the user submits a message that starts an agent turn. Once liquidation starts, later messages cannot add another impulse. The retained wave mode loops continuously while the empty session is idle and stops when agent generation begins. Timing is controlled in `logo-animation.ts` by `ENTRANCE_FPS`, `ENTRANCE_INTERVAL_MS`, `DOT_BUILD_END_FRAME`, `ARROW_ENTRY_START_FRAME`, and `ARROW_ENTRY_END_FRAME`.

Render every animation frame as `.txt`, `.svg`, and `.png`, plus a PNG contact sheet, with:

```sh
cd ~/code/personal/pi/fancy-intro
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
