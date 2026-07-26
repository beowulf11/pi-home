import { execFileSync } from "node:child_process";
import { readFileSync, statSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import {
	DefaultPackageManager,
	getAgentDir,
	SettingsManager,
	VERSION,
	type ExtensionAPI,
	type ExtensionContext,
} from "@earendil-works/pi-coding-agent";
import { truncateToWidth, visibleWidth } from "@earendil-works/pi-tui";
import { generateDefaultWaveFrames } from "./default-wave-animation.ts";
import { FluidTransport } from "./fluid-transport.ts";
import { defaultWaveDimensions, EDITOR_FOOTER_ROWS } from "./intro-layout.ts";
import { resolveIntroProfile } from "./intro-config.ts";
import { rasterToAscii } from "./raster-to-ascii.ts";
import {
	animateLogoEntrance,
	ENTRANCE_END_FRAME,
	ENTRANCE_INTERVAL_MS,
} from "./logo-animation.ts";
import { generateSvgAsciiFrames } from "./svg-ascii-animation.ts";

const EXTENSION_DIR = dirname(fileURLToPath(import.meta.url));
const SOURCE_PATH = join(EXTENSION_DIR, "source.png");
const FALLBACK_PATH = join(EXTENSION_DIR, "ascii-art.txt");
const FLUID_EXECUTABLE = join(EXTENSION_DIR, "bin", "fluid-intro");
const UPDATE_WIDGET_KEY = "custom-intro-updates";
const PI_RELEASE_URL = "https://pi.dev/api/latest-version";
const UPDATE_TIMEOUT_MS = 10_000;
const ASCII_MAP = " .:-=+*#%@";
const CONVERTER = "ascii-image-converter";
const DEFAULT_MAX_LOGO_WIDTH = 48;
const DEFAULT_MAX_LOGO_ROWS = 20;
const MAX_VISIBLE_PACKAGE_UPDATES = 4;
const LOGO_COLOR = "\x1b[38;2;242;137;84m";
const RESET_FOREGROUND = "\x1b[39m";

interface PiRelease {
	version: string;
}

interface ConvertedArt {
	key: string;
	lines: string[];
	simple: boolean;
}

let cachedArt: ConvertedArt | undefined;
let cachedAnimation: { key: string; frames: string[][] } | undefined;

function runConverter(sizeFlag: "-W" | "-H", size: number, simple: boolean): string[] {
	const detailArgs = simple ? ["-m", ASCII_MAP] : ["--complex"];
	const output = execFileSync(
		CONVERTER,
		[SOURCE_PATH, sizeFlag, String(size), ...detailArgs],
		{
			encoding: "utf8",
			stdio: ["ignore", "pipe", "ignore"],
			timeout: 3_000,
		},
	);
	return output.replaceAll("\r\n", "\n").split("\n").filter((_, index, lines) => {
		return index < lines.length - 1 || lines[index] !== "";
	});
}

function verticalMargin(terminalRows: number): number {
	if (terminalRows < 32) return 1;
	if (terminalRows < 44) return 2;
	return 4;
}

function logoLimits(terminalWidth: number, terminalRows: number): { width: number; rows: number } {
	if (terminalWidth >= 220 && terminalRows >= 70) return { width: 96, rows: 40 };
	if (terminalWidth >= 140 && terminalRows >= 52) return { width: 72, rows: 30 };
	return { width: DEFAULT_MAX_LOGO_WIDTH, rows: DEFAULT_MAX_LOGO_ROWS };
}

function imageToAscii(
	terminalWidth: number,
	terminalRows: number,
	updateRows: number,
	margin: number,
): ConvertedArt | undefined {
	try {
		const mtime = statSync(SOURCE_PATH).mtimeMs;
		const limits = logoLimits(terminalWidth, terminalRows);
		const widthLimit = Math.max(2, Math.min(limits.width, Math.floor(terminalWidth * 0.78)));
		const rowLimit = Math.max(
			2,
			Math.min(
				limits.rows,
				terminalRows - margin * 2 - EDITOR_FOOTER_ROWS - updateRows,
			),
		);
		const simple = Math.min(widthLimit, rowLimit * 2) < 40;
		const cacheKey = `${mtime}:${widthLimit}:${rowLimit}:${simple}`;
		if (cachedArt?.key === cacheKey) return cachedArt;

		let lines = runConverter("-W", widthLimit, simple);
		if (lines.length > rowLimit) lines = runConverter("-H", rowLimit, simple);
		cachedArt = { key: cacheKey, lines, simple };
		return cachedArt;
	} catch {
		return undefined;
	}
}

function isNewerVersion(candidate: string, current: string): boolean {
	const parse = (version: string): number[] | undefined => {
		const match = version.trim().match(/^v?(\d+)\.(\d+)\.(\d+)/);
		return match ? [Number(match[1]), Number(match[2]), Number(match[3])] : undefined;
	};
	const next = parse(candidate);
	const installed = parse(current);
	if (!next || !installed) return false;
	for (let index = 0; index < 3; index += 1) {
		if (next[index] !== installed[index]) return (next[index] ?? 0) > (installed[index] ?? 0);
	}
	return false;
}

function fallbackAscii(): string[] {
	try {
		return readFileSync(FALLBACK_PATH, "utf8").replaceAll("\r\n", "\n").split("\n");
	} catch {
		return ["pi"];
	}
}

function centerLogoLine(line: string, canvasWidth: number, terminalWidth: number): string {
	const safeCanvasWidth = Math.min(canvasWidth, terminalWidth);
	const safeLine = truncateToWidth(line, safeCanvasWidth, "");
	const trailingPadding = Math.max(0, safeCanvasWidth - visibleWidth(safeLine));
	const blockPadding = Math.max(0, Math.floor((terminalWidth - safeCanvasWidth) / 2));
	return `${" ".repeat(blockPadding)}${LOGO_COLOR}${safeLine}${" ".repeat(trailingPadding)}${RESET_FOREGROUND}`;
}

async function latestPiVersion(): Promise<string | undefined> {
	try {
		const response = await fetch(PI_RELEASE_URL, {
			headers: { accept: "application/json" },
			signal: AbortSignal.timeout(UPDATE_TIMEOUT_MS),
		});
		if (!response.ok) return undefined;
		const release = (await response.json()) as Partial<PiRelease>;
		if (typeof release.version !== "string") return undefined;
		return isNewerVersion(release.version, VERSION) ? release.version : undefined;
	} catch {
		return undefined;
	}
}

async function availablePackageUpdates(cwd: string, projectTrusted: boolean): Promise<string[]> {
	try {
		const settingsManager = SettingsManager.create(cwd, getAgentDir(), { projectTrusted });
		const packageManager = new DefaultPackageManager({
			cwd,
			agentDir: getAgentDir(),
			settingsManager,
		});
		const updates = await packageManager.checkForAvailableUpdates();
		return updates.map((update) => update.displayName);
	} catch {
		return [];
	}
}

async function suppressBuiltInPackageUpdateNotice(): Promise<void> {
	try {
		const packageEntry = import.meta.resolve("@earendil-works/pi-coding-agent");
		const interactiveModuleUrl = new URL("./modes/interactive/interactive-mode.js", packageEntry);
		const module = (await import(interactiveModuleUrl.href)) as {
			InteractiveMode?: { prototype?: Record<string, unknown> };
		};
		const prototype = module.InteractiveMode?.prototype;
		if (prototype && typeof prototype.checkForPackageUpdates === "function") {
			prototype.checkForPackageUpdates = async () => [];
		}
	} catch {
		// Pi's internal path may change. The custom check still works; only the
		// built-in package notice may also appear until this compatibility shim is updated.
	}
}

export default async function customIntro(pi: ExtensionAPI) {
	const userSkippedPiCheck = Boolean(process.env.PI_SKIP_VERSION_CHECK);
	const userSkippedPackageCheck = Boolean(process.env.PI_SKIP_PACKAGE_UPDATE_CHECK);

	// Pi exposes a switch for its own version notice but not its package notice.
	// Replace both built-in notices so updates appear only in our minimal widget.
	process.env.PI_SKIP_VERSION_CHECK = "1";
	await suppressBuiltInPackageUpdateNotice();

	let startupUpdatesVisible = false;
	let startupUpdateRows = 0;
	let finishIntroAnimation: (() => void) | undefined;

	const hideStartupUpdates = (ctx: ExtensionContext) => {
		startupUpdatesVisible = false;
		startupUpdateRows = 0;
		if (ctx.mode === "tui") ctx.ui.setWidget(UPDATE_WIDGET_KEY, undefined);
	};

	pi.on("input", (_event, ctx) => {
		hideStartupUpdates(ctx);
	});

	pi.on("agent_start", (_event, ctx) => {
		hideStartupUpdates(ctx);
		finishIntroAnimation?.();
	});

	pi.on("session_shutdown", () => {
		finishIntroAnimation?.();
		finishIntroAnimation = undefined;
	});

	pi.on("session_start", (_event, ctx) => {
		if (ctx.mode !== "tui") return;
		startupUpdatesVisible = true;
		startupUpdateRows = 0;
		finishIntroAnimation?.();
		const shouldAnimate = !ctx.sessionManager
			.getBranch()
			.some((entry) => entry.type === "message");
		const introProfile = resolveIntroProfile(ctx.cwd);

		ctx.ui.setHeader((tui, _theme) => {
			const continuouslyAnimate = shouldAnimate && introProfile.animation === "default-wave";
			let animationActive = shouldAnimate;
			let entranceFrame = shouldAnimate ? 0 : ENTRANCE_END_FRAME;
			let timer: ReturnType<typeof setInterval> | undefined;
			let disposed = false;
			let fluidTransport: FluidTransport | undefined;
			let fluidAscii: { sequence: number; width: number; rows: number; lines: string[] } | undefined;
			const finish = () => {
				animationActive = false;
				if (timer) clearInterval(timer);
				timer = undefined;
				entranceFrame = ENTRANCE_END_FRAME;
				fluidTransport?.stop();
				if (!disposed) tui.requestRender();
			};
			finishIntroAnimation = finish;
			if (shouldAnimate) {
				timer = setInterval(() => {
					entranceFrame += 1;
					if (!continuouslyAnimate && entranceFrame >= ENTRANCE_END_FRAME) {
						finish();
						return;
					}
					// The Go process drives renders once fluid frames arrive. Keep this
					// timer only as the one-shot logo clock or procedural-wave fallback.
					if (!continuouslyAnimate || !fluidTransport?.latestFrame) tui.requestRender();
				}, ENTRANCE_INTERVAL_MS);
				timer.unref?.();
			}

			return {
			render(width: number): string[] {
				const margin = verticalMargin(tui.terminal.rows);
				let converted: ConvertedArt | undefined;
				let animationFrames: string[][] | undefined;
				let art: string[];

				if (introProfile.animation === "praktik-entry") {
					converted = imageToAscii(
						width,
						tui.terminal.rows,
						startupUpdateRows,
						margin,
					);
					art = converted?.lines
						?? fallbackAscii().map((line) => truncateToWidth(line, width, ""));
				} else {
					const dimensions = defaultWaveDimensions(
						width,
						tui.terminal.rows,
						startupUpdateRows,
						margin,
					);
					const animationKey = `${introProfile.animation}:${dimensions.width}:${dimensions.rows}`;
					if (cachedAnimation?.key !== animationKey) {
						cachedAnimation = {
							key: animationKey,
							frames: generateDefaultWaveFrames(dimensions.width, dimensions.rows),
						};
					}
					animationFrames = cachedAnimation.frames;
					art = animationFrames.at(-1) ?? ["~"];

					if (continuouslyAnimate && animationActive) {
						const pixelWidth = dimensions.width;
						const pixelHeight = dimensions.rows * 2;
						if (!fluidTransport) {
							fluidTransport = new FluidTransport(FLUID_EXECUTABLE, () => {
								if (!disposed && animationActive) tui.requestRender();
							});
							fluidTransport.start(pixelWidth, pixelHeight);
						} else {
							fluidTransport.resize(pixelWidth, pixelHeight);
						}
					}
					const latest = fluidTransport?.latestFrame;
					if (latest && (latest.sequence !== fluidAscii?.sequence
						|| dimensions.width !== fluidAscii.width
						|| dimensions.rows !== fluidAscii.rows)) {
						fluidAscii = {
							sequence: latest.sequence,
							width: dimensions.width,
							rows: dimensions.rows,
							lines: rasterToAscii(
								latest.pixels,
								latest.width,
								latest.height,
								dimensions.width,
								dimensions.rows,
								{ previous: fluidAscii?.lines },
							),
						};
					}
					if (fluidAscii) art = fluidAscii.lines;
				}

				const reservedRows = EDITOR_FOOTER_ROWS + startupUpdateRows;
				const flexiblePadding = Math.max(
					0,
					tui.terminal.rows - art.length - reservedRows - margin * 2,
				);
				const topPadding = margin + Math.floor(flexiblePadding / 2);
				const bottomPadding = margin + Math.ceil(flexiblePadding / 2);
				const artCanvasWidth = art.reduce(
					(maximum, line) => Math.max(maximum, visibleWidth(line)),
					0,
				);
				let renderedArt = art;
				if (shouldAnimate) {
					if (introProfile.animation === "default-wave") {
						const fallbackFrame = animationFrames && animationFrames.length > 0
							? entranceFrame % animationFrames.length
							: 0;
						renderedArt = fluidAscii?.lines ?? animationFrames?.[fallbackFrame] ?? art;
					} else if (converted) {
						try {
							const logoMtime = statSync(join(EXTENSION_DIR, "logo.svg")).mtimeMs;
							const animationKey = `${introProfile.id}:${converted.key}:${logoMtime}`;
							if (cachedAnimation?.key !== animationKey) {
								const canvasWidth = art.reduce(
									(maximum, line) => Math.max(maximum, [...line].length),
									0,
								);
								cachedAnimation = {
									key: animationKey,
									frames: generateSvgAsciiFrames(canvasWidth, art.length, converted.simple),
								};
							}
							renderedArt = cachedAnimation.frames[
								Math.min(entranceFrame, cachedAnimation.frames.length - 1)
							] ?? art;
						} catch {
							renderedArt = animateLogoEntrance(art, entranceFrame);
						}
					} else {
						renderedArt = animateLogoEntrance(art, entranceFrame);
					}
				}
				return [
					...Array.from({ length: topPadding }, () => ""),
					...renderedArt.map((line) => centerLogoLine(line, artCanvasWidth, width)),
					...Array.from({ length: bottomPadding }, () => ""),
				];
			},
			invalidate() {
				cachedArt = undefined;
				cachedAnimation = undefined;
			},
			dispose() {
				disposed = true;
				finish();
				fluidTransport?.dispose();
				if (finishIntroAnimation === finish) finishIntroAnimation = undefined;
			},
		};
		});

		ctx.ui.setWidget(UPDATE_WIDGET_KEY, undefined);
		if (process.env.PI_OFFLINE) return;

		void Promise.all([
			userSkippedPiCheck ? Promise.resolve(undefined) : latestPiVersion(),
			userSkippedPackageCheck
				? Promise.resolve([])
				: availablePackageUpdates(ctx.cwd, ctx.isProjectTrusted()),
		]).then(([piVersion, packages]) => {
			if (!startupUpdatesVisible) return;
			const lines: string[] = [];
			if (piVersion) lines.push(`Pi update available: ${piVersion}`);
			if (packages.length > 0) {
				const visiblePackages = packages.slice(0, MAX_VISIBLE_PACKAGE_UPDATES);
				lines.push("Package updates:", ...visiblePackages.map((name) => `- ${name}`));
				if (packages.length > visiblePackages.length) {
					lines.push(`- +${packages.length - visiblePackages.length} more`);
				}
			}
			startupUpdateRows = lines.length;
			ctx.ui.setWidget(
				UPDATE_WIDGET_KEY,
				lines.length > 0
					? (_tui, theme) => ({
							render: (width) => lines.map((line) =>
								truncateToWidth(theme.fg("accent", line), width, "")),
							invalidate() {},
						})
					: undefined,
			);
		});
	});
}
