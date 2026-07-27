import { homedir } from "node:os";
import { isAbsolute, relative, resolve } from "node:path";

export type IntroAnimationId =
	| "default-wave"
	| "praktik-entry"
	| "fluid-logo-gather"
	| "galaxy-logo-on-input";

export interface IntroProfile {
	id: "default" | "praktik" | "experiment";
	animation: IntroAnimationId;
	roots: string[];
}

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
	},
	{
		id: "praktik",
		animation: "praktik-entry",
		roots: [PRAKTIK_ROOT],
	},
];

export const DEFAULT_INTRO_PROFILE: IntroProfile = {
	id: "default",
	animation: "galaxy-logo-on-input",
	roots: [],
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
