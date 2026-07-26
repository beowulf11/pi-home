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
	});

	pi.on("session_start", (_event, ctx) => {
		if (ctx.mode !== "tui") return;
		startupUpdatesVisible = true;
		startupUpdateRows = 0;

		ctx.ui.setHeader((tui, _theme) => ({
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
				return [
					...Array.from({ length: topPadding }, () => ""),
					...art.map((line) => centerLine(`${LOGO_COLOR}${line}${RESET_FOREGROUND}`, width)),
					...Array.from({ length: bottomPadding }, () => ""),
				];
			},
			invalidate() {
				cachedArt = undefined;
			},
		}));

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
