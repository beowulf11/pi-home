export const EDITOR_FOOTER_ROWS = 8;

/**
 * The default fluid animation owns the full header canvas. Only the adaptive
 * outer margin and Pi's editor/footer area are excluded.
 */
export function defaultWaveDimensions(
	terminalWidth: number,
	terminalRows: number,
	updateRows: number,
	margin: number,
): { width: number; rows: number } {
	return {
		width: Math.max(1, Math.floor(terminalWidth)),
		rows: Math.max(
			1,
			Math.floor(terminalRows) - EDITOR_FOOTER_ROWS - Math.max(0, updateRows) - Math.max(0, margin) * 2,
		),
	};
}
