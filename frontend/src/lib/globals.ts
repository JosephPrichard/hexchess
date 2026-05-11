export type SelectEvent = "SELECT" | "DESELECT"

export const selectEvents: Record<number, SelectEvent> = {
	0: 'SELECT', // mouse left
	2: 'DESELECT' // mouse right
}