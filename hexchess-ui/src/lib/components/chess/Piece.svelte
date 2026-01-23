<script lang="ts">
	import { onMount } from 'svelte';
	import { hexHeight, hexWidth } from '$lib/components/chess/render';
	import { isPieceWhite, piecenames, pieces, promotions } from '$lib/utils/chess.js';
	import { type SelectEvent, selectEvents } from '$lib/globals';
	import { BadPromotion, type BadPromotionType, type Promotion } from '$lib/components/chess/types';

	export interface PieceProps {
		piece: number;
		isSelected?: boolean;
		isAnnotatable?: boolean;
		isDraggable?: boolean;
		isTransparent?: boolean;
		isPromoting?: boolean;
		nextPromotion?: number;
		initialLeft: number;
		initialTop: number;
		onSelectHexagon?: (key: SelectEvent) => void;
		onSelectPiece?: () => void;
		onDeSelectPiece?: () => void;
		onDragPiece?: (x: number, y: number) => void;
		onDropPiece?: (x: number, y: number) => void;
		onCompletePromotion?: (promotedPiece: Promotion | BadPromotionType) => void;
	}

	let { 
		piece, 
		isSelected,
		isAnnotatable,
		isDraggable,
		isTransparent,
		isPromoting,
		nextPromotion,
		initialTop,
		initialLeft,
		onSelectHexagon,
		onSelectPiece,
		onDeSelectPiece,
		onDragPiece,
		onDropPiece,
		onCompletePromotion
	}: PieceProps = $props();

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
	});

	function onMouseDown(e: MouseEvent) {
		e.preventDefault();
		if (!element || selectEvents[e.button] === undefined) return;

		if (onSelectHexagon) {
			onSelectHexagon(selectEvents[e.button]);
		}
		if (e.button == 2) return; // mouse right click is already used for annotations, so prevent drag

		if (!isDraggable) {
			if (isSelected) {
				onDeSelectPiece?.();
			} else {
				onSelectPiece?.();
			}
			return;
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

	function onMouseDownGlobal(e: MouseEvent) {
		if (!(e.target as HTMLElement).closest("#promotions")) {
			onCompletePromotion?.(BadPromotion);
		}
	}

	onMount(() => {
		document.addEventListener('mousemove', onMove);
		document.addEventListener('mouseup', onMouseUpDrag);
		document.addEventListener('mousedown', onMouseDownGlobal);
		return () => {
			document.removeEventListener('mousemove', onMove);
			document.removeEventListener('mouseup', onMouseUpDrag);
			document.removeEventListener('mousedown', onMouseDownGlobal);
		}
	});

	const whitePromotions = [[pieces.whiteQueen, promotions.queen], [pieces.whiteRook, promotions.rook], [pieces.whiteBishop, promotions.bishop], [pieces.whiteKnight, promotions.knight]];
	const blackPromotions = [[pieces.blackQueen, promotions.queen], [pieces.blackRook, promotions.rook], [pieces.blackBishop, promotions.bishop], [pieces.blackKnight, promotions.knight]];
	const promotePieces = $derived.by(() => isPieceWhite(piece) ? whitePromotions : blackPromotions);

	const fmtPieceURL = (piece: number) => `/pieces/${piecenames[piece]}.png`;
	const pieceImageURL = $derived.by(() => fmtPieceURL(piece));

	function hexStyle(left: number, top: number) {
		return `
			left: ${hexWidth / 8 + left}px;
			top: ${top}px;
			width: ${hexWidth * 0.9}px;
			height: ${hexHeight * 0.9}px;`;
	}
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
	style={hexStyle(initialLeft, initialTop)}
	oncontextmenu={e => e.preventDefault()}
>
	<img class="piece-img-inner" style:opacity={isTransparent ? "0.4" : 1} src={pieceImageURL} alt="" draggable={false} />
</div>
<div
	class="piece-img"
	role="cell"
	tabindex="0"
	style={hexStyle(initialLeft, initialTop)}
	oncontextmenu={e => e.preventDefault()}
>
	{#if nextPromotion !== undefined}
		<img class="piece-img-inner" style:opacity="0.4" src={fmtPieceURL(nextPromotion)} alt="" draggable={false} />
	{/if}
</div>
<div
	bind:this={element}
	role="none"
	class="piece-img"
	style={hexStyle(xOff, yOff)}
	style:z-index={dragging ? "101" : "100"}
	oncontextmenu={onRightClick}
>
	<img role="none" onmousedown={onMouseDown} onmouseup={onMouseUpRelease} class="piece-img-inner" src={pieceImageURL} alt="" draggable={false} />
	{#if isPromoting}
		<div
			id="promotions"
			class="promotion-wrapper"
			style:left="{hexWidth / 8 + xOff}px"
			style:top="{yOff}px"
		>
			{#each promotePieces as [piece, promotion]}
				<img onclick={() => onCompletePromotion?.({piece, kind:promotion})} role="none" class="piece-img-inner promotion-item" src={fmtPieceURL(piece)} alt="" draggable={false} />
			{/each}
		</div>
	{/if}
</div>

<style>
	.promotion-wrapper {
		z-index: 1000;
		width: fit-content;
		background-color: rgb(42, 42, 42);
		border: 3px solid rgb(60, 60, 60);
		border-radius: 6px;
	}

	.promotion-item {
        display: block;
		border-radius: 3px;
        padding: 0 !important;
		margin: 0 !important;
	}

	.promotion-item:hover {
		background-color: rgb(60,60,60);
	}

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
        /*transition: all 0.5s ease-out;*/
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