import assert from "node:assert/strict";
import { test } from "node:test";
import { rasterToAccentMask, rasterToAscii, rasterToLandMask } from "./raster-to-ascii.ts";

test("resamples to exact requested dimensions", () => {
	const lines = rasterToAscii(Uint8Array.from([0, 255, 255, 0]), 2, 2, 7, 3, { density: " .#" });
	assert.equal(lines.length, 3);
	assert.ok(lines.every((line) => [...line].length === 7));
});

test("maps dark and bright density endpoints", () => {
	assert.deepEqual(rasterToAscii(Uint8Array.from([0, 255]), 2, 1, 2, 1, { density: " .#", characterAspect: 1 }), [" #"]);
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
