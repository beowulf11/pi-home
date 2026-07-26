import assert from "node:assert/strict";
import { test } from "node:test";
import { defaultWaveDimensions, EDITOR_FOOTER_ROWS } from "./intro-layout.ts";

test("default wave occupies the full available header canvas", () => {
	assert.deepEqual(defaultWaveDimensions(120, 40, 0, 2), {
		width: 120,
		rows: 40 - EDITOR_FOOTER_ROWS - 4,
	});
});

test("update rows and adaptive margins are reserved from the canvas", () => {
	assert.deepEqual(defaultWaveDimensions(72, 30, 3, 1), {
		width: 72,
		rows: 30 - EDITOR_FOOTER_ROWS - 3 - 2,
	});
});

test("tiny dimensions remain valid", () => {
	assert.deepEqual(defaultWaveDimensions(0, 4, 10, 4), { width: 1, rows: 1 });
});
