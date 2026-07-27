import assert from "node:assert/strict";
import { test } from "node:test";
import {
	COSMIC_DENSITY,
	EXPRESSIVE_DENSITY,
	rasterToAccentMask,
	rasterToAscii,
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

test("prior glyph provides deterministic temporal hysteresis", () => {
	const pixels = Uint8Array.from([120]);
	assert.deepEqual(rasterToAscii(pixels, 1, 1, 1, 1, { density: " .#", previous: ["."] }), ["."]);
});
