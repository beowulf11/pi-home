import { homedir } from "node:os";
import { isAbsolute, relative, resolve } from "node:path";

export type IntroAnimationId =
	| "default-wave"
	| "praktik-entry"
	| "fluid-logo-gather"
	| "galaxy-logo-on-input";

export type GalaxyStyle = "classic" | "living";
export type GalaxyTransition = "direct" | "comet";
export type GalaxyEffect = "nebula" | "starfield" | "shooting-stars" | "pulse";

export interface GalaxyVariant {
	id: string;
	style: GalaxyStyle;
	transition: GalaxyTransition;
	effects: readonly GalaxyEffect[];
}

export interface IntroProfile {
	id: "default" | "praktik" | "experiment";
	animation: IntroAnimationId;
	roots: string[];
	/** Presets eligible for this profile. One is picked once per displayed intro. */
	galaxyVariants?: readonly string[];
}

export const GALAXY_PRESETS: Readonly<Record<string, GalaxyVariant>> = {
	"classic-drift": {
		id: "classic-drift", style: "classic", transition: "direct", effects: ["starfield"],
	},
	"living-nebula": {
		id: "living-nebula", style: "living", transition: "direct", effects: ["nebula", "pulse"],
	},
	"comet-trail": {
		id: "comet-trail", style: "living", transition: "comet", effects: ["starfield"],
	},
	"meteor-shower": {
		id: "meteor-shower", style: "living", transition: "comet", effects: ["starfield", "shooting-stars"],
	},
	"cosmic-storm": {
		id: "cosmic-storm", style: "living", transition: "comet",
		effects: ["nebula", "starfield", "shooting-stars", "pulse"],
	},
};

const DEFAULT_GALAXY_VARIANTS = Object.freeze(Object.keys(GALAXY_PRESETS));
const PRAKTIK_ROOT = resolve(process.env.HOME ?? homedir(), "code", "praktik");
const EXPERIMENT_ROOTS = ["/tmp", "/private/tmp"];

/**
 * Hard-coded for now. Put more specific profiles before the default profile;
 * the first root containing the Pi session cwd wins.
 */
export const INTRO_PROFILES: readonly IntroProfile[] = [
	{
		id: "experiment",
		animation: "galaxy-logo-on-input",
		roots: EXPERIMENT_ROOTS,
		galaxyVariants: DEFAULT_GALAXY_VARIANTS,
	},
	{
		id: "praktik",
		animation: "galaxy-logo-on-input",
		roots: [PRAKTIK_ROOT],
		galaxyVariants: DEFAULT_GALAXY_VARIANTS,
	},
];

export const DEFAULT_INTRO_PROFILE: IntroProfile = {
	id: "default",
	animation: "galaxy-logo-on-input",
	roots: [],
	galaxyVariants: DEFAULT_GALAXY_VARIANTS,
};

function isInsideRoot(cwd: string, root: string): boolean {
	const pathFromRoot = relative(resolve(root), resolve(cwd));
	return pathFromRoot === "" || (!pathFromRoot.startsWith("..") && !isAbsolute(pathFromRoot));
}

export function resolveIntroProfile(cwd: string): IntroProfile {
	return INTRO_PROFILES.find((profile) =>
		profile.roots.some((root) => isInsideRoot(cwd, root)))
		?? DEFAULT_INTRO_PROFILE;
}

const STYLES = new Set<GalaxyStyle>(["classic", "living"]);
const TRANSITIONS = new Set<GalaxyTransition>(["direct", "comet"]);
const EFFECTS = new Set<GalaxyEffect>(["nebula", "starfield", "shooting-stars", "pulse"]);

/**
 * Resolves either a named preset or a composable path such as
 * `living/comet/nebula+starfield+pulse`. Unknown or duplicate modules reject
 * the whole path so a typo can never silently change an intro.
 */
export function parseGalaxyVariant(path: string): GalaxyVariant | undefined {
	const preset = GALAXY_PRESETS[path];
	if (preset) return { ...preset, effects: [...preset.effects] };
	const [styleName, transitionName, effectNames = "", ...extra] = path.split("/");
	if (extra.length > 0 || !STYLES.has(styleName as GalaxyStyle)
		|| !TRANSITIONS.has(transitionName as GalaxyTransition)) return undefined;
	const effects = effectNames === "" ? [] : effectNames.split("+");
	if (new Set(effects).size !== effects.length
		|| effects.some((effect) => !EFFECTS.has(effect as GalaxyEffect))) return undefined;
	return {
		id: path,
		style: styleName as GalaxyStyle,
		transition: transitionName as GalaxyTransition,
		effects: effects as GalaxyEffect[],
	};
}

/** Pick one route once per session, or honor PI_GALAXY_VARIANT/a supplied path. */
export function resolveGalaxyVariant(
	profile: IntroProfile,
	requestedPath: string | undefined = process.env.PI_GALAXY_VARIANT,
	random: () => number = Math.random,
): GalaxyVariant {
	if (requestedPath) {
		const requested = parseGalaxyVariant(requestedPath);
		if (requested) return requested;
	}
	const choices = profile.galaxyVariants?.length
		? profile.galaxyVariants
		: DEFAULT_GALAXY_VARIANTS;
	const index = Math.min(choices.length - 1, Math.floor(Math.max(0, random()) * choices.length));
	return parseGalaxyVariant(choices[index]!) ?? parseGalaxyVariant("living-nebula")!;
}
