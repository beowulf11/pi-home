import { homedir } from "node:os";
import { isAbsolute, relative, resolve } from "node:path";

export type IntroAnimationId = "default-wave" | "praktik-entry";

export interface IntroProfile {
	id: "default" | "praktik";
	animation: IntroAnimationId;
	roots: string[];
}

const PRAKTIK_ROOT = resolve(homedir(), "code", "praktik");

/**
 * Hard-coded for now. Put more specific profiles before the default profile;
 * the first root containing the Pi session cwd wins.
 */
export const INTRO_PROFILES: readonly IntroProfile[] = [
	{
		id: "praktik",
		animation: "praktik-entry",
		roots: [PRAKTIK_ROOT],
	},
];

export const DEFAULT_INTRO_PROFILE: IntroProfile = {
	id: "default",
	animation: "default-wave",
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
