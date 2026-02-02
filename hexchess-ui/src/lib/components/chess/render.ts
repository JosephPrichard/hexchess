import { iterBoard, pieces, ranksPerFile } from '$lib/service/chess';
import type { Hex } from '$lib/api/models';
import type { ChessBoard } from '$lib/pb/messages';

export const moveElementHeight = 35;

export const hexHeight = 56;
export const hexWidth = hexHeight * 1.15;

export const leftPlus = hexWidth * 0.55;
export const leftMinus = hexWidth * -0.55;

export const topOffset = (isWhite: boolean) => (isWhite ? -10 : 35);

export const verticalFileOffsets = [5, 4, 3, 2, 1, 0, 1, 2, 3, 4, 5];
export const ranksBeforeHalf = [0, 1, 2, 3, 4, 5];
export const ranksAfterHalf = [6, 7, 8, 9, 10];
export const halfRank = 5;

export const bgColors = ['rgb(255, 207, 159)', 'rgb(233, 172, 112)', 'rgb(210,140,69)'];
export const mediumPurple = 'rgba(150, 100, 150, 0.4)';
export const darkGreen = 'rgba(100, 111, 64, 0.6)';
export const lime = 'rgb(205, 210, 106, 0.5)';
export const lightGreen = 'rgb(245, 246, 130, 0.5)';
export const white = 'rgb(240, 240, 240, 0.9)';
export const grey = 'rgba(120, 90, 60, 0.5)'
export const hexColorsOffset = [0, 1, 2, 0, 1, 2, 1, 0, 2, 1, 0];

export const maxRank = 10;
export const viewportWidth = (11 * hexWidth) * 0.8; // number of hexagons multiplied by a constant I "eyed"
export const viewportHeight = 11 * hexHeight + hexHeight / 2; // number of hexagons, plus space for the file markers

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

export function hexPoints(left: number, top: number, w: number, h: number) {
	return hexPts
		.map(([x, y]) => {
			const px = left + (x / baseW) * w;
			const py = top  + (y / baseH) * h;
			return `${px},${py}`;
		})
		.join(' ');
}

export function hexPathPoints(left: number, top: number, w: number, h: number) {
	const pts = hexPts.map(([x, y]) => ({
		x: left + (x / baseW) * w,
		y: top  + (y / baseH) * h,
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

export function getTop(file: number, rank: number, isWhitePerspective: boolean): number {
	let top = rank *
		hexHeight +
		(verticalFileOffsets[file] * hexHeight) / 2;
	top = isWhitePerspective ?
		maxRank * hexHeight - top :
		top;
	return maxRank + top;
}

export function getLeft(file: number, isWhitePerspective = true): number {
	file = isWhitePerspective ? file : 10 - file;
	return 5 + file * (hexWidth / 1.33);
}

export function getRank(
	top: number,
	file: number,
	isWhitePerspective: boolean | undefined
): number {
	top = isWhitePerspective
		? maxRank * hexHeight - top
		: top;
	const offset = (verticalFileOffsets[file] * hexHeight) / 2;
	return Math.floor((top - offset + (isWhitePerspective ? hexHeight : 0)) / hexHeight);
}

export function getFile(left: number, isWhitePerspective = true): number {
	const file = left / (hexHeight - 7);
	return isWhitePerspective ? Math.floor(file) : Math.ceil(10 - file);
}

export function findHex(element: HTMLElement | undefined, isWhitePerspective: boolean | undefined, x: number, y: number) {
	if (!element) return undefined;

	const rect = element.getBoundingClientRect();
	const file = getFile(x - rect.left, isWhitePerspective);
	const rank = getRank(y - rect.top, file, isWhitePerspective);

	if (file < 0 || file > ranksPerFile.length || rank < 0 || rank > ranksPerFile[file]) return undefined;

	return { file: file, rank: rank };
}

export function findHexColor(hex: Hex) {
	return (hexColorsOffset[hex.file] + hex.rank) % 3;
}

let GlobalPieceKey = 0;

export type PlacedPiece = {piece: number, file: number, rank: number};

type KeyedHex = {key: number, file: number, rank: number};

export function findKeyedPieces(boardState: ChessBoard, prevPieces?: [number, PlacedPiece][]) {
	const nextPieces: Map<number, PlacedPiece> = new Map();
	if (prevPieces !== undefined) {
		const table: Map<number, KeyedHex[]> = new Map();
		for (const [key, value] of prevPieces) {
			const hexagons = table.get(value.piece);
			const record = {key, file: value.file, rank: value.rank};
			if (hexagons === undefined) {
				table.set(value.piece, [record])
			} else {
				hexagons.push(record);
			}
		}
		iterBoard(boardState, (file, rank, piece) => {
			if (piece === pieces.empty) return;
			const hexagons = table.get(piece);
			if (!hexagons) {
				nextPieces.set(GlobalPieceKey++, {file, rank, piece});
				return;
			}
			const recordIdx = hexagons.findIndex(r => r.file === file && r.rank === rank);
			if (recordIdx !== -1) {
				nextPieces.set(hexagons[recordIdx].key, {file, rank, piece});
				hexagons.splice(recordIdx, 1);
				return;
			}
			const record = hexagons.pop()
			if (record) {
				nextPieces.set(record.key, {file, rank, piece});
				return;
			}
			nextPieces.set(GlobalPieceKey++, {file, rank, piece});
		})
	} else {
		iterBoard(boardState, (file, rank, piece) => {
			if (piece !== pieces.empty) {
				nextPieces.set(GlobalPieceKey++, {file, rank, piece});
			}
		});
	}
	return Array.from(nextPieces.entries());
}