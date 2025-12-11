export function typedEntries<T extends object>(obj: T): {
	[K in keyof T]: [K, T[K]]
}[keyof T][] {
	return Object.entries(obj) as any;
}