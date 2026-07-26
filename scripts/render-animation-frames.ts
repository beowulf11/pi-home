import { execFileSync } from "node:child_process";
import { mkdirSync, rmSync, writeFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { ENTRANCE_END_FRAME } from "../logo-animation.ts";
import { generateSvgAsciiFrames } from "../svg-ascii-animation.ts";

const extensionDir = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const argument = (name: string, fallback: string): string => {
	const index = process.argv.indexOf(name);
	return index >= 0 && process.argv[index + 1] ? process.argv[index + 1]! : fallback;
};
const height = Math.max(4, Number.parseInt(argument("--height", "20"), 10));
const simple = process.argv.includes("--simple");
const outputDir = resolve(argument(
	"--out",
	join(extensionDir, "frames", `h${height}${simple ? "-simple" : "-complex"}`),
));
const sourcePath = join(extensionDir, "source.png");
const orange = "#F28954";
const background = "#282C34";
// Menlo 16px advances by ~9.64px; matching it keeps the fixed ASCII canvas centered.
const characterWidth = 9.64;
const lineHeight = 20;
const padding = 16;

const escapeXml = (text: string): string => text
	.replaceAll("&", "&amp;")
	.replaceAll("<", "&lt;")
	.replaceAll(">", "&gt;");

const detailArgs = simple ? ["-m", " .:-=+*#%@"] : ["--complex"];
const ascii = execFileSync("ascii-image-converter", [sourcePath, "-H", String(height), ...detailArgs], {
	encoding: "utf8",
}).replaceAll("\r\n", "\n").replace(/\n$/, "").split("\n");
const width = Math.max(...ascii.map((line) => [...line].length));
const animationFrames = generateSvgAsciiFrames(width, ascii.length, simple);
const imageWidth = width * characterWidth + padding * 2;
const imageHeight = ascii.length * lineHeight + padding * 2;

rmSync(outputDir, { recursive: true, force: true });
mkdirSync(outputDir, { recursive: true });
const pngPaths: string[] = [];
for (let frame = 0; frame <= ENTRANCE_END_FRAME; frame += 1) {
	const lines = animationFrames[frame] ?? animationFrames.at(-1) ?? ascii;
	const text = lines.map((line, row) =>
		`  <text x="${padding}" y="${padding + (row + 0.8) * lineHeight}">${escapeXml(line)}</text>`).join("\n");
	const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${imageWidth}" height="${imageHeight}" viewBox="0 0 ${imageWidth} ${imageHeight}">
  <rect width="100%" height="100%" fill="${background}"/>
  <g fill="${orange}" font-family="Menlo,Monaco,monospace" font-size="16" xml:space="preserve">
${text}
  </g>
</svg>\n`;
	const basename = `frame-${String(frame).padStart(2, "0")}`;
	const svgPath = join(outputDir, `${basename}.svg`);
	const txtPath = join(outputDir, `${basename}.txt`);
	const pngPath = join(outputDir, `${basename}.png`);
	writeFileSync(svgPath, svg);
	writeFileSync(txtPath, `${lines.join("\n")}\n`);
	execFileSync("magick", [svgPath, pngPath]);
	pngPaths.push(pngPath);
}

execFileSync("magick", ["montage", ...pngPaths, "-tile", "5x", "-geometry", "+8+8", join(outputDir, "contact-sheet.png")]);
console.log(`Rendered ${pngPaths.length} frames and contact-sheet.png to ${outputDir}`);
