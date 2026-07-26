export interface AsciiOptions {
	density?: string;
	/** Terminal cells are normally about twice as tall as they are wide. */
	characterAspect?: number;
	/** Optional prior output enables a small threshold hysteresis. */
	previous?: readonly string[];
}

const DEFAULT_DENSITY = " .:-=+*#%@";

/** Pure, deterministic area resampling from an 8-bit grayscale raster. */
export function rasterToAscii(
	pixels: Uint8Array,
	sourceWidth: number,
	sourceHeight: number,
	targetWidth: number,
	targetHeight: number,
	options: AsciiOptions = {},
): string[] {
	if (!Number.isInteger(sourceWidth) || !Number.isInteger(sourceHeight)
		|| sourceWidth <= 0 || sourceHeight <= 0 || pixels.length !== sourceWidth * sourceHeight) {
		throw new RangeError("raster dimensions do not match pixel data");
	}
	if (!Number.isInteger(targetWidth) || !Number.isInteger(targetHeight)
		|| targetWidth < 0 || targetHeight < 0) throw new RangeError("invalid ASCII dimensions");
	if (targetWidth === 0 || targetHeight === 0) return [];
	const density = options.density ?? DEFAULT_DENSITY;
	if ([...density].length < 2) throw new RangeError("density must contain at least two characters");
	const glyphs = [...density];
	const aspect = Math.max(0.25, options.characterAspect ?? 2);
	const lines: string[] = [];

	for (let y = 0; y < targetHeight; y += 1) {
		let line = "";
		for (let x = 0; x < targetWidth; x += 1) {
			const x0 = Math.floor(x * sourceWidth / targetWidth);
			const x1 = Math.max(x0 + 1, Math.ceil((x + 1) * sourceWidth / targetWidth));
			const centerY = (y + 0.5) * sourceHeight / targetHeight;
			const sampleHeight = sourceHeight / targetHeight * aspect / 2;
			const y0 = Math.max(0, Math.floor(centerY - sampleHeight / 2));
			const y1 = Math.min(sourceHeight, Math.max(y0 + 1, Math.ceil(centerY + sampleHeight / 2)));
			let sum = 0;
			let count = 0;
			for (let sy = y0; sy < y1; sy += 1) {
				for (let sx = x0; sx < Math.min(sourceWidth, x1); sx += 1) {
					sum += pixels[sy * sourceWidth + sx]!;
					count += 1;
				}
			}
			const scaled = (sum / Math.max(1, count) / 255) * (glyphs.length - 1);
			let index = Math.round(scaled);
			const previousGlyph = options.previous?.[y]?.[x];
			const previousIndex = previousGlyph === undefined ? -1 : glyphs.indexOf(previousGlyph);
			if (previousIndex >= 0 && Math.abs(scaled - previousIndex) < 0.6) index = previousIndex;
			line += glyphs[Math.max(0, Math.min(glyphs.length - 1, index))];
		}
		lines.push(line);
	}
	return lines;
}
