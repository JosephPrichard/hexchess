export const hexHeight = 62;
export const hexWidth = hexHeight * 1.2;
export const verticalFileOffsets = [5, 4, 3, 2, 1, 0, 1, 2, 3, 4, 5];
export const colors = ['rgb(255, 207, 159)', 'rgb(233, 172, 112)', 'rgb(210,140,69)'];
export const selectedColor = 'rgba(100, 111, 64, 0.9)';
export const hoveringColor = 'rgba(168, 187, 162, 0.75)'
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
	isWhitePerspective: boolean
): number {
	top = isWhitePerspective
		? 10 * hexHeight - top
		: top;
	const offset = (verticalFileOffsets[file] * hexHeight) / 2;
	return Math.floor((top - offset + hexHeight) / hexHeight);
}


export function getFile(left: number): number {
	return Math.floor(left / (hexHeight - 7));
}