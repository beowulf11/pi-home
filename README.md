# Custom Pi intro

Global Pi extension that:

- converts `source.png` into responsive ASCII art with the Go-based `ascii-image-converter` program;
- centers the art using the current terminal width and height and renders it in `#F28954`;
- regenerates and caches the art when the terminal size or source image changes;
- shows a minimal update list directly above the editor only when updates exist, then removes it as soon as input or agent work begins;
- replaces Pi's built-in version/package update notices to avoid duplicates.

## Artwork

The source image is:

```text
~/.pi/agent/extensions/custom-intro/source.png
```

It currently comes from:

```text
/Users/beowulf/Documents/Projects/Praktik/Graphics/small symbol with padding.png
```

Replacing `source.png` is detected on the next render. Until it exists, `ascii-art.txt` is used as a fallback.

## Converter

Installed globally through Homebrew:

```sh
brew install TheZoraiz/ascii-image-converter/ascii-image-converter
```

The stored source is trimmed to its visible bounds to avoid wasting output space on transparent padding. The extension uses `ascii-image-converter --complex` at normal sizes and falls back to a simpler character map on very small terminals. The logo is capped at 48 columns by 20 rows. Vertical margins adapt to terminal height: one row below 32 terminal rows, two below 44, and four on larger screens. Only the rows used by an actual update notice are reserved, so small screens no longer shrink the logo or add empty space for an absent notice. It first sizes by terminal width and, only when necessary, recalculates by terminal height.

Run `/reload` after changing extension code. The global `quietStartup` setting hides Pi's built-in skills/extensions listing so the intro stays clean. Set `PI_OFFLINE=1` to disable all update checks. Existing `PI_SKIP_VERSION_CHECK=1` and `PI_SKIP_PACKAGE_UPDATE_CHECK=1` settings disable their respective custom checks.
