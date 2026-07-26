import { execFileSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import {
	ARROW_ENTRY_END_FRAME,
	ARROW_ENTRY_START_FRAME,
	DOT_BUILD_END_FRAME,
	ENTRANCE_END_FRAME,
} from "./logo-animation.ts";

const SVG_PATH = new URL("./logo.svg", import.meta.url);
const SIMPLE_MAP = " .:-=+*#%@";
const COMPLEX_MAP = " .'`^\",:;Il!i><~+_-?][}{1)(|\\/tfjrxnuvczXYUJCLQ0OZmwqpdbkhao*#MW&8%B@$";
const LOGO_GRAY = 162;
const VIEWBOX_WIDTH = 1977;
const VIEWBOX_HEIGHT = 1978;
const DOT_CENTER_X = 988.705;
const DOT_CENTER_Y = 989.143;
const ARROW_TRAVEL = 1100;
const RASTER_SCALE = 4;

function easeOutCubic(progress: number): number {
	const clamped = Math.max(0, Math.min(1, progress));
	return 1 - (1 - clamped) ** 3;
}

function logoParts(): string[] {
	const svg = readFileSync(SVG_PATH, "utf8");
	const pathData = svg.match(/<path\s+d="([^"]+)"/)?.[1];
	const parts = pathData?.match(/M[^M]+/g) ?? [];
	if (parts.length !== 5) throw new Error(`Expected five logo components, found ${parts.length}`);
	return parts;
}

function componentSvg(pathData: string, width: number, height: number): string {
	return `<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}" viewBox="0 0 ${VIEWBOX_WIDTH} ${VIEWBOX_HEIGHT}"><path d="${pathData}" fill="#F28954"/></svg>`;
}

function rasterizeComponents(parts: string[], width: number, height: number): Buffer[] {
	const directory = mkdtempSync(join(tmpdir(), "pi-logo-components-"));
	try {
		const paths = parts.map((part, index) => {
			const path = join(directory, `component-${index}.svg`);
			writeFileSync(path, componentSvg(part, width, height));
			return path;
		});
		const raw = execFileSync(
			"magick",
			["-background", "none", ...paths, "-alpha", "extract", "-depth", "8", "gray:-"],
			{ encoding: "buffer", maxBuffer: 16 * 1024 * 1024, timeout: 5_000 },
		);
		const pageBytes = width * height;
		if (raw.length !== pageBytes * parts.length) {
			throw new Error(`Expected ${pageBytes * parts.length} component bytes, received ${raw.length}`);
		}
		return parts.map((_, index) => raw.subarray(index * pageBytes, (index + 1) * pageBytes));
	} finally {
		rmSync(directory, { recursive: true, force: true });
	}
}

function sample(mask: Buffer, width: number, height: number, x: number, y: number): number {
	if (x < 0 || y < 0 || x > width - 1 || y > height - 1) return 0;
	const x0 = Math.floor(x);
	const y0 = Math.floor(y);
	const x1 = Math.min(width - 1, x0 + 1);
	const y1 = Math.min(height - 1, y0 + 1);
	const fx = x - x0;
	const fy = y - y0;
	const top = (mask[y0 * width + x0] ?? 0) * (1 - fx) + (mask[y0 * width + x1] ?? 0) * fx;
	const bottom = (mask[y1 * width + x0] ?? 0) * (1 - fx) + (mask[y1 * width + x1] ?? 0) * fx;
	return top * (1 - fy) + bottom * fy;
}

function alphaToAscii(alpha: number, characterMap: string): string {
	const gray = (alpha * LOGO_GRAY) / 255;
	const index = Math.min(characterMap.length - 1, Math.floor((gray / 255) * characterMap.length));
	return characterMap[index] ?? " ";
}

/**
 * Rasterize the five SVG components once, move those pixel masks at fractional
 * positions inside a fixed clipped viewport, and reconvert the complete image
 * to ASCII for every frame. No pre-existing ASCII character is translated.
 */
export function generateSvgAsciiFrames(width: number, height: number, simple: boolean): string[][] {
	if (width <= 0 || height <= 0) return [];
	const rasterWidth = width * RASTER_SCALE;
	const rasterHeight = height * RASTER_SCALE;
	const masks = rasterizeComponents(logoParts(), rasterWidth, rasterHeight);
	const [bottomLeft, bottomRight, dot, topLeft, topRight] = masks;
	const centerX = (DOT_CENTER_X / VIEWBOX_WIDTH) * rasterWidth;
	const centerY = (DOT_CENTER_Y / VIEWBOX_HEIGHT) * rasterHeight;
	const characterMap = simple ? SIMPLE_MAP : COMPLEX_MAP;
	const frames: string[][] = [];

	for (let frame = 0; frame <= ARROW_ENTRY_END_FRAME; frame += 1) {
		const dotProgress = easeOutCubic((frame + 1) / (DOT_BUILD_END_FRAME + 1));
		const dotScale = 0.2 + dotProgress * 0.8;
		const arrowProgress = easeOutCubic(
			(frame - ARROW_ENTRY_START_FRAME)
				/ (ARROW_ENTRY_END_FRAME - ARROW_ENTRY_START_FRAME),
		);
		const horizontalTravel = (ARROW_TRAVEL / VIEWBOX_WIDTH) * rasterWidth * (1 - arrowProgress);
		const verticalTravel = (ARROW_TRAVEL / VIEWBOX_HEIGHT) * rasterHeight * (1 - arrowProgress);
		const alpha = new Float32Array(rasterWidth * rasterHeight);

		for (let y = 0; y < rasterHeight; y += 1) {
			for (let x = 0; x < rasterWidth; x += 1) {
				const dotX = centerX + (x - centerX) / dotScale;
				const dotY = centerY + (y - centerY) / dotScale;
				alpha[y * rasterWidth + x] = Math.max(
					sample(bottomLeft!, rasterWidth, rasterHeight, x + horizontalTravel, y - verticalTravel),
					sample(bottomRight!, rasterWidth, rasterHeight, x - horizontalTravel, y - verticalTravel),
					sample(topLeft!, rasterWidth, rasterHeight, x + horizontalTravel, y + verticalTravel),
					sample(topRight!, rasterWidth, rasterHeight, x - horizontalTravel, y + verticalTravel),
					sample(dot!, rasterWidth, rasterHeight, dotX, dotY),
				);
			}
		}

		const lines: string[] = [];
		for (let cellY = 0; cellY < height; cellY += 1) {
			let line = "";
			for (let cellX = 0; cellX < width; cellX += 1) {
				let coverage = 0;
				for (let subY = 0; subY < RASTER_SCALE; subY += 1) {
					for (let subX = 0; subX < RASTER_SCALE; subX += 1) {
						const x = cellX * RASTER_SCALE + subX;
						const y = cellY * RASTER_SCALE + subY;
						coverage += alpha[y * rasterWidth + x] ?? 0;
					}
				}
				line += alphaToAscii(coverage / (RASTER_SCALE ** 2), characterMap);
			}
			lines.push(line.trimEnd());
		}
		frames.push(lines);
	}

	const finalFrame = frames.at(-1) ?? [];
	while (frames.length <= ENTRANCE_END_FRAME) frames.push(finalFrame);
	return frames;
}
