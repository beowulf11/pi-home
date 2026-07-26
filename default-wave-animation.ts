import { ENTRANCE_END_FRAME } from "./logo-animation.ts";

const WAVE_CHARACTER = "~";
const WAVE_COUNT = 3;
const CYCLES_ACROSS_CANVAS = 1.6;
const ANIMATION_CYCLES = 2;

/**
 * Initial, deliberately simple default animation. It is separate from the
 * Praktik SVG pipeline so it can be replaced without touching branded intros.
 */
export function generateDefaultWaveFrames(width: number, height: number): string[][] {
	if (width <= 0 || height <= 0) return [];
	const frames: string[][] = [];
	const centerY = (height - 1) / 2;
	const amplitude = Math.max(1, (height - WAVE_COUNT) * 0.28);

	for (let frame = 0; frame <= ENTRANCE_END_FRAME; frame += 1) {
		const progress = frame / ENTRANCE_END_FRAME;
		const phase = progress * Math.PI * 2 * ANIMATION_CYCLES;
		const grid = Array.from({ length: height }, () =>
			Array.from({ length: width }, () => " "));

		for (let x = 0; x < width; x += 1) {
			const waveX = width <= 1 ? 0 : (x / (width - 1)) * Math.PI * 2 * CYCLES_ACROSS_CANVAS;
			for (let wave = 0; wave < WAVE_COUNT; wave += 1) {
				const offset = wave - (WAVE_COUNT - 1) / 2;
				const y = Math.round(
					centerY
					+ Math.sin(waveX - phase + offset * 0.55) * amplitude
					+ offset * 1.35,
				);
				if (y >= 0 && y < height) grid[y]![x] = WAVE_CHARACTER;
			}
		}

		frames.push(grid.map((row) => row.join("").trimEnd()));
	}

	return frames;
}
