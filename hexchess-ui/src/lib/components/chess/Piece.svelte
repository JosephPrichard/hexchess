<script lang="ts">
	import { onMount } from 'svelte';
	import { hexHeight, hexWidth } from '../../services/render';
	import { piecenames } from '../../services/chess';
	import { type SelectEvent, selectEvents } from '../../globals';

	export interface PieceProps {
		piece: number;
		isSelected?: boolean;
		isAnnotatable?: boolean;
		isDraggable?: boolean;
		isTransparent?: boolean;
		initialLeft: number;
		initialTop: number;
		onSelectHexagon?: (key: SelectEvent) => void;
		onSelectPiece?: () => void;
		onDeSelectPiece?: () => void;
		onDragPiece?: (x: number, y: number) => void;
		onDropPiece?: (x: number, y: number) => void;
	}

	let { piece, isSelected, isAnnotatable, isDraggable, isTransparent, initialTop, initialLeft,
		onSelectHexagon, onSelectPiece, onDeSelectPiece, onDragPiece, onDropPiece }: PieceProps = $props();

	let element: HTMLDivElement | undefined;

	let isAnnotated = $state(false);
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
		e.preventDefault();
		if (!element || selectEvents[e.button] === undefined) {
			return
		}

		if (onSelectHexagon) {
			onSelectHexagon(selectEvents[e.button]);
		}
		if (e.button == 2) {
			// mouse right click is already used for annotations, so prevent drag
			return;
		}

		if (!isDraggable) {
			if (isSelected) {
				onDeSelectPiece?.();
			} else {
				onSelectPiece?.();
			}
			return
		}
		onSelectPiece?.();

		const rect = element.getBoundingClientRect();
		xOff += (e.clientX - (rect.x + hexWidth / 3));
		yOff += (e.clientY - (rect.y + hexHeight / 2));
		lastX = e.clientX;
		lastY = e.clientY;
		dragging = true;
	}

	function onMove(e: MouseEvent) {
		if (!element) {
			return
		}
		if (!isDraggable) {
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
			onDragPiece?.(e.clientX, e.clientY);
		}
	}

	function onMouseUpDrag(e: MouseEvent) {
		if (!isDraggable) {
			return
		}
		if (dragging) {
			if (Math.abs(xOff - initialLeft) > hexWidth / 2 || Math.abs(yOff - initialTop) > hexHeight / 2) {
				onDeSelectPiece?.();
			}
			dragging = false;
			xOff = initialLeft;
			yOff = initialTop;
			onDropPiece?.(e.clientX, e.clientY);
		}
	}

	function onMouseUpRelease(e: MouseEvent) {
		e.preventDefault();
		if (isDraggable || e.button != 0) {
			return
		}
	}

	function onRightClick(e: MouseEvent) {
		e.preventDefault();
		if (isAnnotatable) {
			isAnnotated = !isAnnotated;
		}
	}

	onMount(() => {
		document.addEventListener('mousemove', onMove);
		document.addEventListener('mouseup', onMouseUpDrag);
		return () => {
			document.removeEventListener('mousemove', onMove);
			document.removeEventListener('mouseup', onMouseUpDrag);
		}
	});
</script>

<div
	class="annotation"
	class:annotation-show={isAnnotated}
	role="cell"
	tabindex="0"
	style:left="{hexWidth / 8 + initialLeft}px"
	style:top="{hexWidth / 24 + initialTop}px"
	style:width="{hexWidth * 0.75}px"
	style:height="{hexWidth * 0.75}px"
	oncontextmenu={e => e.preventDefault()}
></div>
<div
	class="piece-img"
	role="cell"
	tabindex="0"
	style:left="{hexWidth / 8 + initialLeft}px"
	style:top="{initialTop}px"
	style:width="{hexWidth * 0.9}px"
	style:height="{hexHeight * 0.9}px"
	oncontextmenu={e => e.preventDefault()}
>
	<img
		class="piece-img-inner"
		style:opacity={isTransparent ? "0.4" : 1}
		src="/pieces/{piecenames[piece]}.png"
		alt=""
		draggable={false}
	/>
</div>
<div
	bind:this={element}
	role="none"
	class="piece-img"
	style:left="{hexWidth / 8 + xOff}px"
	style:top="{yOff}px"
	style:width="{hexWidth * 0.9}px"
	style:height="{hexHeight * 0.9}px"
	style:z-index={dragging ? "101" : "100"}
	oncontextmenu={onRightClick}
>
	<img
		role="none"
		onmousedown={onMouseDown}
		onmouseup={onMouseUpRelease}
		class="piece-img-inner"
		src="/pieces/{piecenames[piece]}.png"
		alt=""
		draggable={false}
	/>
</div>

<style>
    .annotation {
        z-index: 100;
        position: absolute;
        border-radius: 50%;
        border: 4px solid red;
        background: transparent;
        box-sizing: border-box;
        opacity: 0;
        transition: opacity 0.15s ease-in-out;
    }

	.annotation-show {
		opacity: 1;
	}

    .piece-img {
		z-index: 100;
        position: absolute;
        user-select: none;
        -moz-user-select: none;
        -webkit-user-select: none;
    }

    .piece-img-inner {
		position: relative;
        cursor: pointer;
        max-width: 100%;
        max-height: 100%;
        user-select: none;
        -moz-user-select: none;
        -webkit-user-select: none;
    }
</style>