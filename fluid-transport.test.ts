import assert from "node:assert/strict";
import { test } from "node:test";
import { FluidTransport, parseFrameLine } from "./fluid-transport.ts";

test("parses a complete frame", () => {
	const frame = parseFrameLine("frame 7 2 2 AAECAw==");
	assert.deepEqual(frame && { ...frame, pixels: [...frame.pixels] }, {
		sequence: 7, width: 2, height: 2, pixels: [0, 1, 2, 3],
	});
});

test("rejects malformed and dimension-mismatched frames", () => {
	for (const line of ["noise", "frame x 1 1 AA==", "frame 1 2 2 AA==", "frame 1 0 1 AA==", "frame 1 1 1 !!!="]) {
		assert.equal(parseFrameLine(line), undefined);
	}
});

test("transport retains only newest sequence and ignores output after dispose", () => {
	const seen: number[] = [];
	const transport = new FluidTransport("missing", (frame) => seen.push(frame.sequence));
	const consume = (transport as unknown as { consume(chunk: string): void }).consume.bind(transport);
	consume("frame 2 1 1 Ag==\nframe 1 1 1 AQ==\n");
	assert.equal(transport.latestFrame?.sequence, 2);
	assert.deepEqual(seen, [2]);
	transport.dispose();
	consume("frame 3 1 1 Aw==\n");
	assert.equal(transport.latestFrame?.sequence, 2);
});
