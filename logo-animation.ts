export const ENTRANCE_INTERVAL_MS = 90;
export const DOT_BUILD_END_FRAME = 7;
export const ARROW_ENTRY_START_FRAME = 5;
export const ARROW_ENTRY_END_FRAME = 25;
export const ENTRANCE_END_FRAME = 28;

type Point = { x: number; y: number };

function pointKey(point: Point): string {
	return `${point.x}:${point.y}`;
}

function findCenterDot(grid: string[][]): Point[] {
	const height = grid.length;
	const width = grid.reduce((maximum, row) => Math.max(maximum, row.length), 0);
	const visited = new Set<string>();
	const components: Point[][] = [];

	// Four-way connectivity keeps the center circle separate even at tiny
	// resolutions where antialiased dot and arrow characters touch diagonally.
	for (let y = 0; y < height; y += 1) {
		for (let x = 0; x < width; x += 1) {
			const start = { x, y };
			if ((grid[y]?.[x] ?? " ") === " " || visited.has(pointKey(start))) continue;
			const points: Point[] = [];
			const queue = [start];
			visited.add(pointKey(start));
			while (queue.length > 0) {
				const point = queue.pop()!;
				points.push(point);
				for (const [dx, dy] of [[-1, 0], [1, 0], [0, -1], [0, 1]] as const) {
					const neighbor = { x: point.x + dx, y: point.y + dy };
					if (neighbor.x < 0 || neighbor.y < 0 || neighbor.x >= width || neighbor.y >= height) continue;
					if ((grid[neighbor.y]?.[neighbor.x] ?? " ") === " ") continue;
					const key = pointKey(neighbor);
					if (visited.has(key)) continue;
					visited.add(key);
					queue.push(neighbor);
				}
			}
			components.push(points);
		}
	}

	const centerX = (width - 1) / 2;
	const centerY = (height - 1) / 2;
	return components.sort((left, right) => {
		const nearestDistance = (component: Point[]) => Math.min(...component.map((point) =>
			Math.hypot(point.x - centerX, (point.y - centerY) * 2)));
		return nearestDistance(left) - nearestDistance(right);
	})[0] ?? [];
}

function easeOutCubic(progress: number): number {
	const clamped = Math.max(0, Math.min(1, progress));
	return 1 - (1 - clamped) ** 3;
}

// Math.round(-n.5) rounds toward zero while Math.round(n.5) rounds away from
// zero. That made opposite arrows differ by one cell during translation.
function symmetricRound(value: number): number {
	return Math.sign(value) * Math.round(Math.abs(value));
}

export function animateLogoEntrance(lines: string[], frame: number): string[] {
	if (frame >= ARROW_ENTRY_END_FRAME) return lines;
	const width = lines.reduce((maximum, line) => Math.max(maximum, [...line].length), 0);
	const height = lines.length;
	if (width === 0 || height === 0) return lines;

	const grid = lines.map((line) => {
		const row = [...line];
		return [...row, ...Array.from({ length: width - row.length }, () => " ")];
	});
	const output = Array.from({ length: height }, () => Array.from({ length: width }, () => " "));
	const dotPoints = findCenterDot(grid);
	const dotKeys = new Set(dotPoints.map(pointKey));
	const centerX = (width - 1) / 2;
	const centerY = (height - 1) / 2;

	// Reveal by radius rather than by point count. Cells in the same radial shell
	// appear together, so the first dot frame remains centered even on even grids.
	const dotDistances = dotPoints.map((point) =>
		Math.hypot(point.x - centerX, (point.y - centerY) * 2));
	const maximumDotDistance = Math.max(0, ...dotDistances);
	const dotProgress = easeOutCubic(frame / DOT_BUILD_END_FRAME);
	const minimumDotDistance = dotDistances.length > 0 ? Math.min(...dotDistances) : 0;
	const dotRadius = frame === 0
		? minimumDotDistance + 0.01
		: Math.max(minimumDotDistance + 0.01, maximumDotDistance * dotProgress);
	for (let index = 0; index < dotPoints.length; index += 1) {
		if ((dotDistances[index] ?? Infinity) > dotRadius + 0.001) continue;
		const point = dotPoints[index]!;
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
				const translatedX = x + symmetricRound(startX * (1 - arrowProgress));
				const translatedY = y + symmetricRound(startY * (1 - arrowProgress));
				if (translatedX < 0 || translatedY < 0 || translatedX >= width || translatedY >= height) continue;
				output[translatedY]![translatedX] = character;
			}
		}
	}

	return output.map((row) => row.join("").trimEnd());
}
