<script lang="ts">
	import { piecenames } from '$lib/utils/globals';

	export interface PiecePanelProps {
		isWhite: boolean
	}

	const { isWhite }: PiecePanelProps = $props();

	const pieces = $derived.by(() => isWhite ?
		[1, 3, 5, 7, 9, 11] : // pawn, knight, bishop, rook, queen, king
		[2, 4, 6, 8, 10, 12]); // pawn, knight, bishop, rook, queen, king
</script>

<style>

</style>