import assert from "node:assert/strict";
import { test } from "node:test";
import {
	COSMIC_DENSITY,
	EXPRESSIVE_DENSITY,
	rasterToAccentMask,
	rasterToAscii,
	rasterToBrailleSurfaces,
	rasterToLandMask,
	remap3dSurfaceGlyphs,
	remapCometGlyphs,
} from "./raster-to-ascii.ts";

test("resamples to exact requested dimensions", () => {
	const lines = rasterToAscii(Uint8Array.from([0, 255, 255, 0]), 2, 2, 7, 3, { density: " .#" });
	assert.equal(lines.length, 3);
	assert.ok(lines.every((line) => [...line].length === 7));
});

test("maps dark and bright density endpoints", () => {
	assert.deepEqual(rasterToAscii(Uint8Array.from([0, 255]), 2, 1, 2, 1, { density: " .#", characterAspect: 1 }), [" #"]);
});

test("expressive ramp provides fine-grained Unicode shading", () => {
	assert.ok([...EXPRESSIVE_DENSITY].length > 80);
	for (const glyph of ["⠁", "╱", "╲", "░", "▒", "▓", "⣿"]) {
		assert.ok(EXPRESSIVE_DENSITY.includes(glyph));
	}
	const shades = rasterToAscii(
		Uint8Array.from([0, 64, 128, 192, 255]),
		5,
		1,
		5,
		1,
		{ density: EXPRESSIVE_DENSITY, characterAspect: 1 },
	)[0]!;
	assert.equal([...new Set(shades)].length, 5);
	assert.equal(shades[0], " ");
	assert.equal(shades.at(-1), "⣿");
});

test("cosmic ramp and comet labels use dedicated Unicode glyphs", () => {
	assert.ok(COSMIC_DENSITY.includes("✧") && COSMIC_DENSITY.includes("⋆"));
	const sourceGlyph = [...COSMIC_DENSITY].at(-2)!;
	const [remapped] = remapCometGlyphs([sourceGlyph.repeat(3)], [[0, 128, 255]]);
	const glyphs = [...remapped!];
	assert.equal(glyphs[0], sourceGlyph);
	assert.notEqual(glyphs[1], sourceGlyph);
	assert.notEqual(glyphs[2], sourceGlyph);
	assert.notEqual(glyphs[1], glyphs[2]);
});

test("3-D surfaces receive distinct glyph vocabularies at equal brightness", () => {
	const sourceGlyph = [...EXPRESSIVE_DENSITY][Math.floor([...EXPRESSIVE_DENSITY].length * .25)]!;
	const [remapped] = remap3dSurfaceGlyphs(
		[sourceGlyph.repeat(4)],
		[[0, 32, 64, 96]],
	);
	const glyphs = [...remapped!];
	assert.equal(glyphs[0], sourceGlyph);
	assert.equal(new Set(glyphs).size, 4);
	assert.ok(" .·┄─╱╲┆│┊┃┏┓┗┛░▒▓█".includes(glyphs[2]!));

	const cosmicGlyph = [...COSMIC_DENSITY][10]!;
	const [cosmicRemap] = remap3dSurfaceGlyphs(
		[cosmicGlyph.repeat(2)],
		[[0, 32]],
		COSMIC_DENSITY,
	);
	assert.equal([...cosmicRemap!][0], cosmicGlyph);
	assert.notEqual([...cosmicRemap!][1], cosmicGlyph);
});

test("rejects malformed raster input", () => {
	assert.throws(() => rasterToAscii(new Uint8Array(3), 2, 2, 1, 1), /dimensions/);
	assert.throws(() => rasterToAscii(new Uint8Array(1), 1, 1, -1, 1), /dimensions/);
});

test("resamples land material independently from character density", () => {
	assert.deepEqual(
		rasterToLandMask(Uint8Array.from([0, 255, 255, 255]), 2, 2, 2, 2, 1),
		[[false, true], [true, true]],
	);
});

test("preserves the strongest comet accent in each terminal cell", () => {
	assert.deepEqual(
		rasterToAccentMask(Uint8Array.from([0, 128, 0, 255]), 2, 2, 1, 1),
		[[255]],
	);
});

test("Braille uses Unicode's spatial 2×4 dot ordering", () => {
	for (const [dot, bit] of [1, 8, 2, 16, 4, 32, 64, 128].entries()) {
		const pixels = new Uint8Array(8);
		const surfaces = new Uint8Array(8);
		pixels[dot] = 255;
		surfaces[dot] = 32;
		const result = rasterToBrailleSurfaces(pixels, surfaces, 2, 4, [" "]);
		assert.deepEqual(result.lines, [String.fromCodePoint(0x2800 + bit)]);
		assert.deepEqual(result.brightness, [[1]]);
	}
});

test("solid Braille preserves normal-based lighting without eroding geometry", () => {
	for (const label of [32, 64, 96]) {
		for (const light of [64, 128, 255]) {
			const result = rasterToBrailleSurfaces(new Uint8Array(8).fill(light),
				new Uint8Array(8).fill(label), 2, 4, ["*"]);
			assert.deepEqual(result.lines, ["⣿"]);
			assert.ok(Math.abs(result.brightness[0]![0]! - light / 255) < 1e-12);
		}
	}
});

test("Braille leaves scenery and comet cells unchanged and rejects invalid data", () => {
	for (const label of [0, 128, 255]) {
		assert.deepEqual(rasterToBrailleSurfaces(new Uint8Array(8).fill(255),
			new Uint8Array(8).fill(label), 2, 4, ["✦"]),
			{ lines: ["✦"], brightness: [[undefined]] });
	}
	assert.deepEqual(rasterToBrailleSurfaces(new Uint8Array(8).fill(8),
		new Uint8Array(8).fill(32), 2, 4, [" "]).lines, [" "]);
	assert.throws(() => rasterToBrailleSurfaces(new Uint8Array(8), new Uint8Array(7), 2, 4, [" "]), /dimensions/);
});

test("whole-scene Braille shares spatial geometry and preserves scene lighting", () => {
	for (const label of [0, 128, 255]) {
		const result = rasterToBrailleSurfaces(Uint8Array.from([255, 0, 64, 0, 128, 0, 32, 0]),
			new Uint8Array(8).fill(label), 2, 4, ["✦"], { includeScene: true, ditherScene: false });
		assert.deepEqual(result.lines, ["⡇"]);
		const expected = [255, 64, 128, 32].reduce((sum, value) => sum + Math.pow(value / 255, .7), 0) / 4;
		assert.ok(Math.abs(result.brightness[0]![0]! - expected) < 1e-12);
	}
	assert.deepEqual(rasterToBrailleSurfaces(new Uint8Array(8), new Uint8Array(8),
		2, 4, ["✦"], { includeScene: true }).lines, [" "]);
});

test("scene shading distinguishes faint haze, arms, and bright cores without changing dots", () => {
	for (const label of [0, 128, 255]) {
		const shades = [24, 64, 128, 255].map((intensity) => {
			const result = rasterToBrailleSurfaces(new Uint8Array(8).fill(intensity),
				new Uint8Array(8).fill(label), 2, 4, [" "], { includeScene: true, ditherScene: false });
			assert.deepEqual(result.lines, ["⣿"]);
			return result.brightness[0]![0]!;
		});
		assert.ok(shades[0]! < shades[1]! && shades[1]! < shades[2]! && shades[2]! < shades[3]!);
		assert.equal(shades[3], 1);
		assert.ok(shades[1]! > 64 / 255, "gamma lift keeps midtones readable");
	}
});

test("scene dot density follows intensity without dithering the solid logo", () => {
	const width = 64, height = 32;
	const base = Array.from({ length: height / 4 }, () => " ".repeat(width / 2));
	const countDots = (lines: string[]) => lines.join("").split("").reduce((sum, glyph) => {
		let bits = glyph === " " ? 0 : glyph.charCodeAt(0) - 0x2800;
		while (bits) { sum += bits & 1; bits >>>= 1; }
		return sum;
	}, 0);
	const counts = [24, 64, 128, 255].map((intensity) => {
		const pixels = new Uint8Array(width * height).fill(intensity);
		const labels = new Uint8Array(width * height);
		const result = rasterToBrailleSurfaces(pixels, labels, width, height, base, { includeScene: true });
		assert.deepEqual(result, rasterToBrailleSurfaces(pixels, labels, width, height, base, { includeScene: true }));
		return countDots(result.lines);
	});
	assert.ok(counts[0]! > 0 && counts[0]! < counts[1]! && counts[1]! < counts[2]! && counts[2]! < counts[3]!);
	assert.ok(counts[1]! < width * height / 3, "dim haze must not become solid ribbons");
	assert.equal(counts[3], width * height);
	assert.equal(countDots(rasterToBrailleSurfaces(new Uint8Array(width * height).fill(24),
		new Uint8Array(width * height).fill(32), width, height, base, { includeScene: true }).lines), width * height);
});

test("prior glyph provides deterministic temporal hysteresis", () => {
	const pixels = Uint8Array.from([120]);
	assert.deepEqual(rasterToAscii(pixels, 1, 1, 1, 1, { density: " .#", previous: ["."] }), ["."]);
});
