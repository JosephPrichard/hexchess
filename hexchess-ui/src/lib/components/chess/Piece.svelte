<script lang="ts">
	import { piecenames } from '$lib/utils/globals.js';
	import type { Hexagon } from '$lib/api/messages';
	import { onMount } from 'svelte';

	export interface PieceProps {
		file: number;
		rank: number;
		piece: number;
		draggable: boolean;
		onClickPiece?: (hex: Hexagon) => void;
	}

	const { file, rank, piece, onClickPiece }: PieceProps = $props();

	let dragging = false;
	let xOff = 0;
	let yOff = 0;
	let lastX = 0;
	let lastY = 0;

	function onKeyDown() {
		dragging = true;
		console.log("Hovering piece", file, rank);
	}

	function onKeyUp() {
		dragging = false;
	}

	onMount(() => {
		function onMove(e: MouseEvent) {
			if (dragging) {
				xOff += lastX - e.clientX;
				yOff += lastY - e.clientY;
				console.log("Moved offset to", xOff, yOff);
			}
		}
		document.addEventListener('mousemove', onMove);
		return () => {
			document.removeEventListener('mousemove', onMove);
		}
	});
</script>

<div
	role="button"
	tabindex="0"
	class="piece-img"
	onmousedown={() => onClickPiece ? onClickPiece({ file, rank }) : {}}
	onkeydown={onKeyUp}
	onkeyup={onKeyDown}
	style:position="relative"
>
	<img class="piece-img-inner" src="/pieces/{piecenames[piece]}.png" alt="" draggable={false} />
</div>

<style>
    .piece-img {
        position: relative;
        top: 2px;
        max-width: 88%;
        max-height: 88%;
        cursor: pointer;
    }

    .piece-img-inner {
        max-width: 100%;
        max-height: 100%;
        cursor: pointer;
    }
</style>