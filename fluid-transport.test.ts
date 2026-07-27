import assert from "node:assert/strict";
import { test } from "node:test";
import { FluidTransport, parseFrameLine } from "./fluid-transport.ts";

test("parses a complete frame", () => {
	const frame = parseFrameLine("frame 7 2 2 AAECAw== /wD/AA==");
	assert.deepEqual(frame && { ...frame, pixels: [...frame.pixels], land: [...frame.land] }, {
		sequence: 7, width: 2, height: 2, pixels: [0, 1, 2, 3], land: [255, 0, 255, 0], phase: undefined,
	});
});

test("parses experiment frame phases", () => {
	assert.equal(parseFrameLine("frame 8 1 1 /w== AA== gather")?.phase, "gather");
	assert.equal(parseFrameLine("frame 8 1 1 /w== AA== unknown"), undefined);
});

test("rejects malformed and dimension-mismatched frames", () => {
	for (const line of [
		"noise",
		"frame x 1 1 AA== AA==",
		"frame 1 2 2 AA== AA==",
		"frame 1 1 1 AA==",
		"frame 1 1 1 AA== !!!=",
	]) {
		assert.equal(parseFrameLine(line), undefined);
	}
});

test("transport retains only newest sequence and ignores output after dispose", () => {
	const seen: number[] = [];
	const transport = new FluidTransport("missing", (frame) => seen.push(frame.sequence));
	const consume = (transport as unknown as { consume(chunk: string): void }).consume.bind(transport);
	consume("frame 2 1 1 Ag== AA==\nframe 1 1 1 AQ== /w==\n");
	assert.equal(transport.latestFrame?.sequence, 2);
	assert.deepEqual(seen, [2]);
	transport.dispose();
	consume("frame 3 1 1 Aw== AA==\n");
	assert.equal(transport.latestFrame?.sequence, 2);
});
