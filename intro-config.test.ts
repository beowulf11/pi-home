import assert from "node:assert/strict";
import { homedir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";
import {
	parseGalaxyVariant,
	resolveGalaxyVariant,
	resolveIntroProfile,
} from "./intro-config.ts";

test("routes temporary directories to the latest experiment", () => {
	assert.equal(resolveIntroProfile("/tmp").id, "experiment");
	assert.equal(resolveIntroProfile("/tmp/pi-intro-test/nested").animation, "galaxy-logo-on-input");
	assert.equal(resolveIntroProfile("/private/tmp/pi-intro-test").id, "experiment");
	assert.equal(resolveIntroProfile("/tmp/pi-intro-test").logoPresentation, "rotating-3d");
	assert.equal(resolveIntroProfile("/tmp-sibling").id, "default");
});

test("parses named and composable galaxy routes", () => {
	assert.deepEqual(parseGalaxyVariant("meteor-shower"), {
		id: "meteor-shower",
		transition: "comet",
		effects: ["starfield", "shooting-stars"],
	});
	assert.deepEqual(parseGalaxyVariant("direct/nebula+starfield"), {
		id: "direct/nebula+starfield",
		transition: "direct",
		effects: ["nebula", "starfield"],
	});
	assert.equal(parseGalaxyVariant("warp/pulse"), undefined);
	assert.equal(parseGalaxyVariant("comet/pulse+pulse"), undefined);
	assert.equal(parseGalaxyVariant("obsolete/direct/nebula"), undefined);
});

test("selects one route or honors an explicit combination", () => {
	const profile = resolveIntroProfile("/tmp/project");
	const firstPreset = resolveGalaxyVariant(profile, undefined, () => 0);
	assert.equal(firstPreset.id, "deep-field");
	assert.equal(firstPreset.transition, "comet");
	assert.equal(resolveGalaxyVariant(profile, undefined, () => .999).id, "cosmic-storm");
	assert.equal(
		resolveGalaxyVariant(profile, "direct/nebula+pulse", () => 0).id,
		"direct/nebula+pulse",
	);
});

test("uses the galaxy animation for every profile", () => {
	const praktikProfile = resolveIntroProfile(join(homedir(), "code", "praktik", "computer"));
	assert.equal(praktikProfile.id, "praktik");
	assert.equal(praktikProfile.animation, "galaxy-logo-on-input");
	assert.equal(praktikProfile.logoPresentation, "rotating-3d");

	const defaultProfile = resolveIntroProfile(join(homedir(), "code", "other"));
	assert.equal(defaultProfile.id, "default");
	assert.equal(defaultProfile.animation, "galaxy-logo-on-input");
	assert.equal(defaultProfile.logoPresentation, "rotating-3d");
});
