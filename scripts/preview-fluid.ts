import { join } from "node:path";
import { FluidTransport, type FluidTransportOptions } from "../fluid-transport.ts";
import { rasterToAccentMask, rasterToAscii, rasterToBrailleSurfaces } from "../raster-to-ascii.ts";

// Ignore the retired whole-logo shading experiment flag.
const args = process.argv.filter((arg) => arg !== "--shade-logo");
const width = Math.max(8, Number(args[2] ?? 48));
const height = Math.max(4, Number(args[3] ?? 16));
const requestedMode = args[4] ?? "galaxy-logo-on-input";
const mode: NonNullable<FluidTransportOptions["mode"]> = requestedMode === "default"
	|| requestedMode === "fluid-logo-gather"
	|| requestedMode === "galaxy-logo-on-input"
	? requestedMode
	: "galaxy-logo-on-input";
const triggerFrame = Math.max(1, Number(args[5] ?? 900));
const transitionEffect = args[6] === "direct" ? "direct" : "comet";
const validEffects = new Set(["nebula", "starfield", "shooting-stars", "pulse"] as const);
const galaxyEffects = (args[7] ?? "nebula,starfield,pulse")
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
		transitionEffect,
		galaxyEffects,
		logoPresentation: "rotating-3d",
	};
const rasterScale = mode === "default" ? 1 : 2;
const transport = new FluidTransport(
	join(import.meta.dirname, "..", "bin", "fluid-intro"),
	(frame) => {
		let lines = rasterToAscii(frame.pixels, frame.width, frame.height, width, height);
		if (mode !== "default") {
			const braille = rasterToBrailleSurfaces(frame.pixels, frame.accent, frame.width, frame.height, lines, { includeScene: true });
			const accents = rasterToAccentMask(frame.accent, frame.width, frame.height, width, height);
			lines = braille.lines.map((line, y) => [...line].map((glyph, x) => {
				const shade = braille.brightness[y]?.[x];
				if (shade === undefined) return glyph;
				const label = accents[y]?.[x];
				const base = label === 255 ? [255, 82, 36] : label === 128 ? [210, 24, 48] : [242, 137, 84];
				const rgb = base.map((channel) => Math.round(channel * Math.max(.25, shade)));
				return `\x1b[38;2;${rgb.join(";")}m${glyph}\x1b[39m`;
			}).join(""));
		}
		process.stdout.write(`\x1b[H\x1b[2J${lines.join("\n")}\n\n${frame.phase ?? "ocean"} · frame ${count}\n`);
		count++;
		if (mode === "galaxy-logo-on-input" && !transitionSent && count >= triggerFrame) {
			transitionSent = true;
			transport.transition();
		}
		if ((frame.phase === "settled" && (transitionSent || mode === "fluid-logo-gather"))
			|| (mode === "default" && count >= triggerFrame)) {
			transport.dispose();
		}
	},
	options,
);
transport.start(width * rasterScale, height * 2 * rasterScale,
	Math.min(width, 96) * rasterScale, Math.min(height, 40) * rasterScale);
process.on("SIGINT", () => {
	transport.dispose();
	process.exit(0);
});
