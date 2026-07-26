import { join } from "node:path";
import { FluidTransport } from "../fluid-transport.ts";
import { rasterToAscii } from "../raster-to-ascii.ts";

const width = Math.max(8, Number(process.argv[2] ?? 48));
const height = Math.max(4, Number(process.argv[3] ?? 16));
let count = 0;
const transport = new FluidTransport(join(import.meta.dirname, "..", "bin", "fluid-intro"), (frame) => {
	const lines = rasterToAscii(frame.pixels, frame.width, frame.height, width, height);
	process.stdout.write(`\x1b[H\x1b[2J${lines.join("\n")}\n`);
	if (++count >= 180) transport.dispose();
});
transport.start(width, height * 2);
process.on("SIGINT", () => { transport.dispose(); process.exit(0); });
