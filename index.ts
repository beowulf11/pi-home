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

const EXTENSION_DIR = dirname(fileURLToPath(import.meta.url));
const SOURCE_PATH = join(EXTENSION_DIR, "source.png");
const FALLBACK_PATH = join(EXTENSION_DIR, "ascii-art.txt");
const UPDATE_WIDGET_KEY = "custom-intro-updates";
const PI_RELEASE_URL = "https://pi.dev/api/latest-version";
const UPDATE_TIMEOUT_MS = 10_000;
const ASCII_MAP = " .:-=+*#%@";
const CONVERTER = "ascii-image-converter";
const MAX_LOGO_WIDTH = 48;
const MAX_LOGO_ROWS = 20;
const EDITOR_FOOTER_ROWS = 8;
const MAX_VISIBLE_PACKAGE_UPDATES = 4;
const LOGO_COLOR = "\x1b[38;2;242;137;84m";
const RESET_FOREGROUND = "\x1b[39m";
const ENTRANCE_INTERVAL_MS = 90;
const DOT_BUILD_END_FRAME = 7;
const ARROW_ENTRY_START_FRAME = 5;
const ARROW_ENTRY_END_FRAME = 25;
const ENTRANCE_END_FRAME = 28;

type Point = { x: number; y: number };

function pointKey(point: Point): string {
	return `${point.x}:${point.y}`;
}

function findCenterComponent(grid: string[][]): Point[] {
	const height = grid.length;
	const width = grid.reduce((maximum, row) => Math.max(maximum, row.length), 0);
	const visited = new Set<string>();
	const components: Point[][] = [];

	for (let y = 0; y < height; y += 1) {
		for (let x = 0; x < width; x += 1) {
			if ((grid[y]?.[x] ?? " ") === " ") continue;
			const start = { x, y };
			if (visited.has(pointKey(start))) continue;

			const component: Point[] = [];
			const queue = [start];
			visited.add(pointKey(start));
			while (queue.length > 0) {
				const point = queue.pop()!;
				component.push(point);
				for (let dy = -1; dy <= 1; dy += 1) {
					for (let dx = -1; dx <= 1; dx += 1) {
						if (dx === 0 && dy === 0) continue;
						const neighbor = { x: point.x + dx, y: point.y + dy };
						if (neighbor.x < 0 || neighbor.y < 0 || neighbor.x >= width || neighbor.y >= height) continue;
						if ((grid[neighbor.y]?.[neighbor.x] ?? " ") === " ") continue;
						const key = pointKey(neighbor);
						if (visited.has(key)) continue;
						visited.add(key);
						queue.push(neighbor);
					}
				}
			}
			components.push(component);
		}
	}

	const centerX = (width - 1) / 2;
	const centerY = (height - 1) / 2;
	return components.sort((left, right) => {
		const distance = (component: Point[]) => {
			const x = component.reduce((sum, point) => sum + point.x, 0) / component.length;
			const y = component.reduce((sum, point) => sum + point.y, 0) / component.length;
			return Math.hypot((x - centerX) / Math.max(1, width), (y - centerY) / Math.max(1, height));
		};
		return distance(left) - distance(right);
	})[0] ?? [];
}

function easeOutCubic(progress: number): number {
	const clamped = Math.max(0, Math.min(1, progress));
	return 1 - (1 - clamped) ** 3;
}

function animateLogoEntrance(lines: string[], frame: number): string[] {
	if (frame >= ARROW_ENTRY_END_FRAME) return lines;
	const width = lines.reduce((maximum, line) => Math.max(maximum, [...line].length), 0);
	const height = lines.length;
	if (width === 0 || height === 0) return lines;

	const grid = lines.map((line) => {
		const row = [...line];
		return [...row, ...Array.from({ length: width - row.length }, () => " ")];
	});
	const output = Array.from({ length: height }, () => Array.from({ length: width }, () => " "));
	const dotPoints = findCenterComponent(grid);
	const dotKeys = new Set(dotPoints.map(pointKey));
	const centerX = (width - 1) / 2;
	const centerY = (height - 1) / 2;

	const orderedDot = [...dotPoints].sort((left, right) => {
		const leftDistance = Math.hypot(left.x - centerX, (left.y - centerY) * 2);
		const rightDistance = Math.hypot(right.x - centerX, (right.y - centerY) * 2);
		return leftDistance - rightDistance;
	});
	const dotProgress = easeOutCubic(frame / DOT_BUILD_END_FRAME);
	const visibleDotCount = Math.max(1, Math.ceil(orderedDot.length * dotProgress));
	for (const point of orderedDot.slice(0, visibleDotCount)) {
		output[point.y]![point.x] = grid[point.y]![point.x]!;
	}

	const arrowProgress = easeOutCubic(
		(frame - ARROW_ENTRY_START_FRAME)
			/ (ARROW_ENTRY_END_FRAME - ARROW_ENTRY_START_FRAME),
	);
	if (arrowProgress > 0) {
		const horizontalTravel = Math.ceil(width * 0.55);
		const verticalTravel = Math.ceil(height * 0.55);
		for (let y = 0; y < height; y += 1) {
			for (let x = 0; x < width; x += 1) {
				const character = grid[y]![x]!;
				if (character === " " || dotKeys.has(pointKey({ x, y }))) continue;
				const startX = x < centerX ? -horizontalTravel : horizontalTravel;
				const startY = y < centerY ? -verticalTravel : verticalTravel;
				const translatedX = x + Math.round(startX * (1 - arrowProgress));
				const translatedY = y + Math.round(startY * (1 - arrowProgress));
				if (translatedX < 0 || translatedY < 0 || translatedX >= width || translatedY >= height) continue;
				output[translatedY]![translatedX] = character;
			}
		}
	}

	return output.map((row) => row.join("").trimEnd());
}

interface PiRelease {
	version: string;
}

let cachedArt: { key: string; lines: string[] } | undefined;

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

function imageToAscii(
	terminalWidth: number,
	terminalRows: number,
	updateRows: number,
	margin: number,
): string[] | undefined {
	try {
		const mtime = statSync(SOURCE_PATH).mtimeMs;
		const widthLimit = Math.max(2, Math.min(MAX_LOGO_WIDTH, Math.floor(terminalWidth * 0.78)));
		const rowLimit = Math.max(
			2,
			Math.min(
				MAX_LOGO_ROWS,
				terminalRows - margin * 2 - EDITOR_FOOTER_ROWS - updateRows,
			),
		);
		const simple = Math.min(widthLimit, rowLimit * 2) < 40;
		const cacheKey = `${mtime}:${widthLimit}:${rowLimit}:${simple}`;
		if (cachedArt?.key === cacheKey) return cachedArt.lines;

		let lines = runConverter("-W", widthLimit, simple);
		if (lines.length > rowLimit) lines = runConverter("-H", rowLimit, simple);
		cachedArt = { key: cacheKey, lines };
		return lines;
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

function centerLine(line: string, width: number): string {
	const safeLine = truncateToWidth(line, width, "");
	const padding = Math.max(0, Math.floor((width - visibleWidth(safeLine)) / 2));
	return `${" ".repeat(padding)}${safeLine}`;
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
	let finishLogoEntrance: (() => void) | undefined;

	const hideStartupUpdates = (ctx: ExtensionContext) => {
		startupUpdatesVisible = false;
		startupUpdateRows = 0;
		finishLogoEntrance?.();
		if (ctx.mode === "tui") ctx.ui.setWidget(UPDATE_WIDGET_KEY, undefined);
	};

	pi.on("input", (_event, ctx) => {
		hideStartupUpdates(ctx);
	});

	pi.on("agent_start", (_event, ctx) => {
		hideStartupUpdates(ctx);
	});

	pi.on("session_start", (_event, ctx) => {
		if (ctx.mode !== "tui") return;
		startupUpdatesVisible = true;
		startupUpdateRows = 0;
		finishLogoEntrance?.();
		const shouldAnimate = !ctx.sessionManager
			.getBranch()
			.some((entry) => entry.type === "message");

		ctx.ui.setHeader((tui, _theme) => {
			let entranceFrame = shouldAnimate ? 0 : ENTRANCE_END_FRAME;
			let timer: ReturnType<typeof setInterval> | undefined;
			const finish = () => {
				if (timer) clearInterval(timer);
				timer = undefined;
				entranceFrame = ENTRANCE_END_FRAME;
				tui.requestRender();
			};
			finishLogoEntrance = finish;
			if (shouldAnimate) {
				timer = setInterval(() => {
					if (ctx.ui.getEditorText().length > 0) {
						finish();
						return;
					}
					entranceFrame += 1;
					if (entranceFrame >= ENTRANCE_END_FRAME) {
						finish();
						return;
					}
					tui.requestRender();
				}, ENTRANCE_INTERVAL_MS);
				timer.unref?.();
			}

			return {
			render(width: number): string[] {
				const margin = verticalMargin(tui.terminal.rows);
				const converted = imageToAscii(
					width,
					tui.terminal.rows,
					startupUpdateRows,
					margin,
				);
				const art = converted
					?? fallbackAscii().map((line) => truncateToWidth(line, width, ""));
				const reservedRows = EDITOR_FOOTER_ROWS + startupUpdateRows;
				const flexiblePadding = Math.max(
					0,
					tui.terminal.rows - art.length - reservedRows - margin * 2,
				);
				const topPadding = margin + Math.floor(flexiblePadding / 2);
				const bottomPadding = margin + Math.ceil(flexiblePadding / 2);
				const renderedArt = animateLogoEntrance(art, entranceFrame);
				return [
					...Array.from({ length: topPadding }, () => ""),
					...renderedArt.map((line) =>
						centerLine(`${LOGO_COLOR}${line}${RESET_FOREGROUND}`, width)),
					...Array.from({ length: bottomPadding }, () => ""),
				];
			},
			invalidate() {
				cachedArt = undefined;
			},
			dispose() {
				finish();
				if (finishLogoEntrance === finish) finishLogoEntrance = undefined;
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
