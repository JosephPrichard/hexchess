import { ranksPerFile } from './chess';

export const hexHeight = 62;
export const hexWidth = hexHeight * 1.2;
export const verticalFileOffsets = [5, 4, 3, 2, 1, 0, 1, 2, 3, 4, 5];
export const colors = ['rgb(255, 207, 159)', 'rgb(233, 172, 112)', 'rgb(210,140,69)'];
export const selectedColor = 'rgba(100, 111, 64, 0.6)';
export const highlightedColor = 'rgb(245, 246, 130, 0.5)';
export const hoveringColor = 'rgb(245, 246, 130, 0.5)'
export const colorsOffset = [0, 1, 2, 0, 1, 2, 1, 0, 2, 1, 0];
export const chessRowHeight = 45;
export const maxChessRows = 12;

export function getTop(
	file: number,
	rank: number,
	isWhitePerspective: boolean
): number {
	const top = rank *
		hexHeight +
		(verticalFileOffsets[file] * hexHeight) / 2;
	return isWhitePerspective ?
		10 * hexHeight - top :
		top;
}

export function getLeft(file: number): number {
	return file * (hexHeight - 7);
}

export function getRank(
	top: number,
	file: number,
	isWhitePerspective: boolean | undefined
): number {
	top = isWhitePerspective
		? 10 * hexHeight - top
		: top;
	const offset = (verticalFileOffsets[file] * hexHeight) / 2;
	return Math.floor((top - offset + (isWhitePerspective ? hexHeight : 0)) / hexHeight);
}


export function getFile(left: number): number {
	return Math.floor(left / (hexHeight - 7));
}

export function findHex(element: HTMLElement | undefined, isWhitePerspective: boolean | undefined, x: number, y: number) {
	if (!element) {
		return;
	}
	const rect = element.getBoundingClientRect();
	const file = getFile(x - rect.left);
	const rank = getRank(y - rect.top, file, isWhitePerspective);
	if (file < 0 || file > ranksPerFile.length || rank < 0 || rank > ranksPerFile[file]) {
		return;
	}
	return { file: file, rank: rank };
}