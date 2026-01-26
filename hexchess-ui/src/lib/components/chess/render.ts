import { ranksPerFile } from '$lib/service/chess';
import type { Hex } from '$lib/api/models';

export const hexHeight = 56;
export const hexWidth = hexHeight * 1.2;
export const verticalFileOffsets = [5, 4, 3, 2, 1, 0, 1, 2, 3, 4, 5];
export const bgColors = ['rgb(255, 207, 159)', 'rgb(233, 172, 112)', 'rgb(210,140,69)'];
export const mediumPurple = 'rgba(150, 100, 150, 0.4)';
export const darkGreen = 'rgba(100, 111, 64, 0.6)';
export const lime = 'rgb(205, 210, 106, 0.5)';
export const lightGreen = 'rgb(245, 246, 130, 0.5)'
export const hexColorsOffset = [0, 1, 2, 0, 1, 2, 1, 0, 2, 1, 0];

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

export function getLeft(file: number, isWhitePerspective = true): number {
	file = isWhitePerspective ? file : 10 - file;
	return file * (hexWidth / 1.4);
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

export function getFile(left: number, isWhitePerspective = true): number {
	const file = left / (hexHeight - 7);
	return isWhitePerspective ? Math.floor(file) : Math.ceil(10 - file);
}

export function findHex(element: HTMLElement | undefined, isWhitePerspective: boolean | undefined, x: number, y: number) {
	if (!element) return;
	const rect = element.getBoundingClientRect();
	const file = getFile(x - rect.left, isWhitePerspective);
	const rank = getRank(y - rect.top, file, isWhitePerspective);
	if (file < 0 || file > ranksPerFile.length || rank < 0 || rank > ranksPerFile[file]) return;
	return { file: file, rank: rank };
}

export function findHexColor(hex: Hex) {
	return (hexColorsOffset[hex.file] + hex.rank) % 3;
}