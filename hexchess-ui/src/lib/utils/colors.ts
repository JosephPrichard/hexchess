export function generateColors(n: number): ((o: number) => string)[] {
	const colors: ((o: number) => string)[] = [];

	for (let i = 0; i < n; i++) {
		const hue = (i * 360) / n; // evenly spaced hues
		const saturation = 70;     // good default
		const lightness = 50;

		colors.push(hslToRgbString(hue, saturation, lightness));
	}

	return colors;
}

export function hslToRgbString(h: number, s: number, l: number): (o: number) => string {
	s /= 100;
	l /= 100;

	const k = (n: number) => (n + h / 30) % 12;
	const a = s * Math.min(l, 1 - l);

	const f = (n: number) =>
		l - a * Math.max(-1, Math.min(k(n) - 3, Math.min(9 - k(n), 1)));

	const r = Math.round(255 * f(0));
	const g = Math.round(255 * f(8));
	const b = Math.round(255 * f(4));

	return (o: number) => `rgb(${r}, ${g}, ${b}, ${o})`;
}