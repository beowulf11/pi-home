import { spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import type {
	GalaxyEffect,
	GalaxyTransition,
	LogoPresentation,
} from "./intro-config.ts";

export type FluidPhase = "ocean" | "galaxy" | "comet" | "impact" | "gather" | "liquidate" | "settled";

export interface FluidFrame {
	sequence: number;
	width: number;
	height: number;
	pixels: Uint8Array;
	/** 255 for visible land, 0 for water/air at each source pixel. */
	land: Uint8Array;
	/** 32/64/96 label 3-D front/edge/back; 128/255 label comet tail/head. */
	accent: Uint8Array;
	phase?: FluidPhase;
}

export interface FluidTransportOptions {
	mode?: "default" | "fluid-logo-gather" | "galaxy-logo-on-input";
	logoPath?: string;
	/** Independently swappable and composable experiment modules. */
	transitionEffect?: GalaxyTransition;
	galaxyEffects?: readonly GalaxyEffect[];
	logoPresentation?: LogoPresentation;
}

export function parseFrameLine(line: string): FluidFrame | undefined {
	const match = line.match(/^frame (\d+) ([1-9]\d*) ([1-9]\d*) ([A-Za-z0-9+/]+={0,2}) ([A-Za-z0-9+/]+={0,2})(?: (ocean|galaxy|comet|impact|gather|liquidate|settled)| ([A-Za-z0-9+/]+={0,2})(?: (ocean|galaxy|comet|impact|gather|liquidate|settled))?)?$/);
	if (!match) return undefined;
	const sequence = Number(match[1]);
	const width = Number(match[2]);
	const height = Number(match[3]);
	if (!Number.isSafeInteger(sequence) || !Number.isSafeInteger(width) || !Number.isSafeInteger(height)) return undefined;
	const encodedPixels = match[4]!;
	const encodedLand = match[5]!;
	const encodedAccent = match[7];
	const pixels = Buffer.from(encodedPixels, "base64");
	const land = Buffer.from(encodedLand, "base64");
	const expectedLength = width * height;
	const accent = encodedAccent
		? Buffer.from(encodedAccent, "base64")
		: Buffer.alloc(expectedLength);
	if (pixels.length !== expectedLength || land.length !== expectedLength || accent.length !== expectedLength
		|| pixels.toString("base64") !== encodedPixels
		|| land.toString("base64") !== encodedLand
		|| (encodedAccent && accent.toString("base64") !== encodedAccent)) return undefined;
	return {
		sequence,
		width,
		height,
		pixels: new Uint8Array(pixels),
		land: new Uint8Array(land),
		accent: new Uint8Array(accent),
		phase: (match[6] ?? match[8]) as FluidPhase | undefined,
	};
}

export class FluidTransport {
	private child: ChildProcessWithoutNullStreams | undefined;
	private buffered = "";
	private disposed = false;
	private lastResize = "";
	private readonly executable: string;
	private readonly onFrame: (frame: FluidFrame) => void;
	private readonly options: FluidTransportOptions;
	latestFrame: FluidFrame | undefined;

	constructor(
		executable: string,
		onFrame: (frame: FluidFrame) => void,
		options: FluidTransportOptions = {},
	) {
		this.executable = executable;
		this.onFrame = onFrame;
		this.options = options;
	}

	start(pixelWidth: number, pixelHeight: number, logoWidth?: number, logoRows?: number): void {
		if (this.disposed || this.child) return;
		try {
			const args = this.options.mode === "fluid-logo-gather"
				|| this.options.mode === "galaxy-logo-on-input"
				? ["--mode", this.options.mode, "--logo", this.options.logoPath ?? ""]
				: [];
			if (this.options.transitionEffect) args.push("--transition-effect", this.options.transitionEffect);
			if (this.options.galaxyEffects?.length) {
				args.push("--galaxy-effects", this.options.galaxyEffects.join(","));
			}
			if (this.options.logoPresentation) {
				args.push("--logo-presentation", this.options.logoPresentation);
			}
			const child = spawn(this.executable, args, { stdio: ["pipe", "pipe", "pipe"] });
			this.child = child;
			child.stdin.on("error", () => {});
			child.stdout.setEncoding("utf8");
			child.stdout.on("data", (chunk: string) => this.consume(chunk));
			// Drain diagnostics without ever forwarding them to the terminal/TUI.
			child.stderr.resume();
			child.on("error", () => {
				if (this.child === child) this.child = undefined;
			});
			child.on("exit", () => {
				if (this.child === child) this.child = undefined;
			});
			this.resize(pixelWidth, pixelHeight, logoWidth, logoRows);
		} catch {
			this.child = undefined;
		}
	}

	resize(pixelWidth: number, pixelHeight: number, logoWidth?: number, logoRows?: number): void {
		const width = Math.max(1, Math.floor(pixelWidth));
		const height = Math.max(1, Math.floor(pixelHeight));
		const targetWidth = logoWidth === undefined ? undefined : Math.max(1, Math.floor(logoWidth));
		const targetRows = logoRows === undefined ? undefined : Math.max(1, Math.floor(logoRows));
		const command = targetWidth !== undefined && targetRows !== undefined
			? `resize ${width} ${height} ${targetWidth} ${targetRows}`
			: `resize ${width} ${height}`;
		if (command === this.lastResize) return;
		this.lastResize = command;
		if (this.child?.stdin.writable) this.child.stdin.write(`${command}\n`);
	}

	private consume(chunk: string): void {
		if (this.disposed) return;
		this.buffered += chunk;
		for (;;) {
			const newline = this.buffered.indexOf("\n");
			if (newline < 0) break;
			const line = this.buffered.slice(0, newline).replace(/\r$/, "");
			this.buffered = this.buffered.slice(newline + 1);
			const frame = parseFrameLine(line);
			if (!frame || frame.sequence <= (this.latestFrame?.sequence ?? -1)) continue;
			this.latestFrame = frame;
			this.onFrame(frame);
		}
		// Bound memory if a broken process emits an unterminated stream.
		if (this.buffered.length > 1_000_000) this.buffered = "";
	}

	transition(): void {
		if (this.child?.stdin.writable) this.child.stdin.write("transition\n");
	}

	stop(): void {
		const child = this.child;
		this.child = undefined;
		if (!child) return;
		if (child.stdin.writable) child.stdin.end("stop\n");
		const killTimer = setTimeout(() => child.kill(), 250);
		killTimer.unref?.();
		child.once("exit", () => clearTimeout(killTimer));
	}

	dispose(): void {
		if (this.disposed) return;
		this.disposed = true;
		this.stop();
	}
}
