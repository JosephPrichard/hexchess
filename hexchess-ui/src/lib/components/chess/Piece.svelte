<script lang="ts">
	import { hexHeight, hexWidth, piecenames } from '$lib/utils/globals.js';
	import { onMount } from 'svelte';

	export interface PieceProps {
		piece: number;
		initialLeft: number;
		initialTop: number;
		draggable: boolean;
		onSelectPiece: () => void;
		onDeSelectPiece: () => void;
	}

	const { piece, draggable, initialTop, initialLeft, onSelectPiece, onDeSelectPiece }: PieceProps = $props();

	let element: HTMLDivElement | undefined;

	let dragging = $state(false);
	let xOff = $state(initialLeft);
	let yOff = $state(initialTop);
	let lastX = 0;
	let lastY = 0;

	$effect(() => {
		xOff = initialLeft;
		yOff = initialTop;
	})

	function onMouseDown(e: MouseEvent) {
		onSelectPiece();

		if (!element || !draggable) {
			return;
		}

		const rect = element.getBoundingClientRect();
		xOff += (e.clientX - (rect.x + hexWidth / 3));
		yOff += (e.clientY - (rect.y + hexHeight / 2));
		lastX = e.clientX;
		lastY = e.clientY;
		dragging = true;
	}

	function onMove(e: MouseEvent) {
		if (!draggable) {
			return
		}
		if (e.clientX >= screen.width || e.clientY >= screen.height) {
			return
		}
		if (dragging) {
			xOff += e.clientX - lastX;
			yOff += e.clientY - lastY;
			lastX = e.clientX;
			lastY = e.clientY;
		}
	}

	function onMouseUp(_: MouseEvent) {
		if (!draggable) {
			onDeSelectPiece();
			return
		}
		if (dragging) {
			if (Math.abs(xOff - initialLeft) > hexWidth / 4 && Math.abs(yOff - initialTop) > hexHeight / 4) {
				onDeSelectPiece();
			}
			dragging = false;
			xOff = initialLeft;
			yOff = initialTop;
		}
	}

	onMount(() => {
		document.addEventListener('mousemove', onMove);
		document.addEventListener('mouseup', onMouseUp);
		return () => {
			document.removeEventListener('mousemove', onMove);
			document.removeEventListener('mouseup', onMouseUp);
		}
	});
</script>

<div
	bind:this={element}
	role="button"
	tabindex="0"
	class="piece-img"
	onmousedown={onMouseDown}
	onmouseup={onMouseUp}
	style:left="{hexWidth / 8 + xOff}px"
	style:top="{yOff}px"
	style:width="{hexWidth * 0.9}px"
	style:height="{hexHeight * 0.9}px"
	style:z-index={dragging ? "101" : "100"}
>
	<img
		class="piece-img-inner"
		src="/pieces/{piecenames[piece]}.png"
		alt=""
		draggable={false}
	/>
</div>

<style>
    .piece-img {
        position: absolute;
        cursor: pointer;
    }

    .piece-img-inner {
		position: relative;
        cursor: pointer;
        max-width: 100%;
        max-height: 100%;
    }
</style>