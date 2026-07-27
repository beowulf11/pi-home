import assert from "node:assert/strict";
import { homedir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";
import { resolveIntroProfile } from "./intro-config.ts";

test("routes temporary directories to the latest experiment", () => {
	assert.equal(resolveIntroProfile("/tmp").id, "experiment");
	assert.equal(resolveIntroProfile("/tmp/pi-intro-test/nested").animation, "galaxy-logo-on-input");
	assert.equal(resolveIntroProfile("/private/tmp/pi-intro-test").id, "experiment");
	assert.equal(resolveIntroProfile("/tmp-sibling").id, "default");
});

test("uses the galaxy animation for every profile", () => {
	const praktikProfile = resolveIntroProfile(join(homedir(), "code", "praktik", "computer"));
	assert.equal(praktikProfile.id, "praktik");
	assert.equal(praktikProfile.animation, "galaxy-logo-on-input");

	const defaultProfile = resolveIntroProfile(join(homedir(), "code", "other"));
	assert.equal(defaultProfile.id, "default");
	assert.equal(defaultProfile.animation, "galaxy-logo-on-input");
});
