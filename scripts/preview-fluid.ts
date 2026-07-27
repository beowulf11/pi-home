import { join } from "node:path";
import { FluidTransport, type FluidTransportOptions } from "../fluid-transport.ts";
import { rasterToAscii } from "../raster-to-ascii.ts";

const width = Math.max(8, Number(process.argv[2] ?? 48));
const height = Math.max(4, Number(process.argv[3] ?? 16));
const requestedMode = process.argv[4] ?? "galaxy-logo-on-input";
const mode: NonNullable<FluidTransportOptions["mode"]> = requestedMode === "default"
	|| requestedMode === "fluid-logo-gather"
	|| requestedMode === "galaxy-logo-on-input"
	? requestedMode
	: "galaxy-logo-on-input";
const triggerFrame = Math.max(1, Number(process.argv[5] ?? 180));
const galaxyStyle = process.argv[6] === "classic" ? "classic" : "living";
const transitionEffect = process.argv[7] === "direct" ? "direct" : "comet";
const validEffects = new Set(["nebula", "starfield", "shooting-stars", "pulse"] as const);
const galaxyEffects = (process.argv[8] ?? "nebula,starfield,pulse")
	.split(",")
	.filter((effect): effect is "nebula" | "starfield" | "shooting-stars" | "pulse" =>
		validEffects.has(effect as "nebula" | "starfield" | "shooting-stars" | "pulse"));
let count = 0;
let transitionSent = false;

const options: FluidTransportOptions = mode === "default"
	? {}
	: {
		mode,
		logoPath: join(import.meta.dirname, "..", "source.png"),
		galaxyStyle,
		transitionEffect,
		galaxyEffects,
	};
const transport = new FluidTransport(
	join(import.meta.dirname, "..", "bin", "fluid-intro"),
	(frame) => {
		const lines = rasterToAscii(frame.pixels, frame.width, frame.height, width, height);
		process.stdout.write(`\x1b[H\x1b[2J${lines.join("\n")}\n\n${frame.phase ?? "ocean"} · frame ${count}\n`);
		count++;
		if (mode === "galaxy-logo-on-input" && !transitionSent && count >= triggerFrame) {
			transitionSent = true;
			transport.transition();
		}
		if (frame.phase === "settled" || (mode === "default" && count >= triggerFrame)) {
			transport.dispose();
		}
	},
	options,
);
transport.start(width, height * 2, Math.min(width, 48), Math.min(height, 20));
process.on("SIGINT", () => {
	transport.dispose();
	process.exit(0);
});
