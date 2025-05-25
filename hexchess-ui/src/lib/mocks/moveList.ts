import type { PieceMove } from '$lib/models';

export const moveList: PieceMove[] = [
	{
		piece: 1,
		from: {
			file: 1,
			rank: 0
		},
		to: {
			file: 1,
			rank: 1
		}
	},
	{
		piece: 2,
		from: {
			file: 1,
			rank: 6
		},
		to: {
			file: 1,
			rank: 5
		}
	},
	{
		piece: 1,
		from: {
			file: 1,
			rank: 1
		},
		to: {
			file: 1,
			rank: 2
		}
	},
	{
		piece: 2,
		from: {
			file: 1,
			rank: 5
		},
		to: {
			file: 1,
			rank: 4
		}
	},
	{
		piece: 1,
		from: {
			file: 1,
			rank: 2
		},
		to: {
			file: 1,
			rank: 3
		}
	},
]
