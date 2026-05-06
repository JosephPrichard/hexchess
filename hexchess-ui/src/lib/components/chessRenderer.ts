import { ranksPerFile } from '$lib/service/chess';
import type { Hex } from '$lib/api/models';

export interface ChessRenderData {
	hexHeight: number;
	topOffset: (isWhite: boolean) => number;
}

export const moveElementHeight = 35;

const hexHeight = 56;

export const defaultRenderArgs = {
	hexHeight: hexHeight,
	topOffset: (isWhite: boolean) => (isWhite ? -10 : 35)
};

export const colors = {
	bgColors: ['rgb(255, 207, 159)', 'rgb(233, 172, 112)', 'rgb(210,140,69)'],
	mediumPurple: 'rgba(150, 100, 150, 0.4)',
	darkGreen: 'rgba(100, 111, 64, 0.6)',
	lime: 'rgb(205, 210, 106, 0.5)',
	lightGreen: 'rgb(245, 246, 130, 0.5)',
	white: 'rgb(240, 240, 240, 0.9)',
	grey: 'rgba(120, 90, 60, 0.5)',
	hexColorsOffset: [0, 1, 2, 0, 1, 2, 1, 0, 2, 1, 0],
};

const verticalFileOffsets = [5, 4, 3, 2, 1, 0, 1, 2, 3, 4, 5];
export const ranksBeforeHalf = [0, 1, 2, 3, 4, 5];
export const ranksAfterHalf = [6, 7, 8, 9, 10];

export const halfRank = 5;
export const maxRank = 10;

const baseW = 200;
const baseH = 173;

const hexPts = [
	[50, 0],
	[150, 0],
	[200, 86.6],
	[150, 173],
	[50, 173],
	[0, 86.6],
];

export class ChessRenderer {
	hexHeight: number;
	hexWidth: number;
	viewportHeight: number;
	viewportWidth: number;
	leftPlus: number;
	leftMinus: number;
	topOffset: (isWhite: boolean) => number;

	constructor(args: ChessRenderData) {
		const hexHeight = args.hexHeight;
		const hexWidth = hexHeight * 1.15;

		const leftPlus = hexWidth * 0.55;
		const leftMinus = hexWidth * -0.55;

		const viewportWidth = (11 * hexWidth) * 0.8; // number of hexagons multiplied by a constant "eyed"
		const viewportHeight = 11 * hexHeight + hexHeight / 2; // number of hexagons, plus space for the file markers

		this.hexHeight = hexHeight;
		this.hexWidth = hexWidth;
		this.viewportHeight = viewportHeight;
		this.viewportWidth = viewportWidth;
		this.leftPlus = leftPlus;
		this.leftMinus = leftMinus;

		this.topOffset = args.topOffset;
	}

	getTop(file: number, rank: number, isWhitePerspective: boolean): number {
		let top = rank *
			this.hexHeight +
			(verticalFileOffsets[file] * this.hexHeight) / 2;
		top = isWhitePerspective ?
			maxRank * this.hexHeight - top :
			top;
		return maxRank + top;
	}

	getLeft(file: number, isWhitePerspective = true): number {
		file = isWhitePerspective ? file : 10 - file;
		return 5 + file * (this.hexWidth / 1.33);
	}

	getRank(top: number, file: number, isWhitePerspective: boolean | undefined): number {
		top = isWhitePerspective
			? maxRank * this.hexHeight - top
			: top;
		const offset = (verticalFileOffsets[file] * this.hexHeight) / 2;
		return Math.floor((top - offset + (isWhitePerspective ? this.hexHeight : 0)) / this.hexHeight);
	}

	getFile(left: number, isWhitePerspective = true): number {
		const file = left / (this.hexHeight - 7);
		return isWhitePerspective ? Math.floor(file) : Math.ceil(10 - file);
	}

	findHex(element: HTMLElement | undefined, isWhitePerspective: boolean | undefined, x: number, y: number) {
		if (!element) return undefined;

		const rect = element.getBoundingClientRect();
		const file = this.getFile(x - rect.left, isWhitePerspective);
		const rank = this.getRank(y - rect.top, file, isWhitePerspective);

		if (file < 0 || file > ranksPerFile.length || rank < 0 || rank > ranksPerFile[file]) return undefined;

		return { file: file, rank: rank };
	}

	findHexColor(hex: Hex) {
		return (colors.hexColorsOffset[hex.file] + hex.rank) % 3;
	}

	hexPoints(left: number, top: number) {
		return hexPts
			.map(([x, y]) => {
				const px = left + (x / baseW) * this.hexWidth;
				const py = top  + (y / baseH) * this.hexHeight;
				return `${px},${py}`;
			})
			.join(' ');
	}

	hexPathPoints(left: number, top: number) {
		const pts = hexPts.map(([x, y]) => ({
			x: left + (x / baseW) * this.hexWidth,
			y: top  + (y / baseH) * this.hexHeight,
		}));

		const len = pts.length;

		const path: string[] = [];

		for (let i = 0; i < len; i++) {
			const prev = pts[(i - 1 + len) % len];
			const curr = pts[i];
			const next = pts[(i + 1) % len];

			const v1x = prev.x - curr.x;
			const v1y = prev.y - curr.y;
			const v2x = next.x - curr.x;
			const v2y = next.y - curr.y;

			const l1 = Math.hypot(v1x, v1y);
			const l2 = Math.hypot(v2x, v2y);

			const p1x = curr.x + (v1x / l1) * 2;
			const p1y = curr.y + (v1y / l1) * 2;
			const p2x = curr.x + (v2x / l2) * 2;
			const p2y = curr.y + (v2y / l2) * 2;

			if (i === 0) path.push(`M ${p1x} ${p1y}`);
			else path.push(`L ${p1x} ${p1y}`);

			path.push(`Q ${curr.x} ${curr.y} ${p2x} ${p2y}`);
		}

		path.push("Z");
		return path.join(" ");
	}
}