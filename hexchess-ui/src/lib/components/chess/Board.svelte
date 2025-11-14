<script lang="ts">
	import type { ChessBoard } from '$lib/api/messages';
	import Piece from '$lib/components/chess/Piece.svelte';
	import type { Hex } from '$lib/api/model';
	import { defaultBoard, isBlack, isWhite, pieces, ranksPerFile } from '$lib/utils/chess';
	import { getLeft, getTop, hexHeight, hexWidth, hoveringColor, selectedColor, colors, colorsOffset, getFile, getRank } from '$lib/utils/render';

	export interface BoardProps {
		board?: ChessBoard;
		draggable?: "turn" | "anyone" | "none";
		isWhitePerspective: boolean;
		selectedHexagon?: Hex;
		potentialMoves?: Hex[];
		onSelectPiece?: (hex: Hex) => void;
		onDeSelectPiece?: (hex: Hex) => void;
		onDropPiece?: (from: Hex, to: Hex, piece: number) => void;
	}

	let element: HTMLDivElement | undefined;

	const { board, draggable, isWhitePerspective, selectedHexagon, potentialMoves, onSelectPiece, onDeSelectPiece, onDropPiece }: BoardProps = $props();

	let hoveringHexagon: Hex | undefined = $state(undefined);

	function onSelectBoardPiece(hex: Hex) {
		onSelectPiece?.(hex);
	}

	function onDeSelectBoardPiece(hex: Hex) {
		onDeSelectPiece?.(hex);
	}

	function getHex(x: number, y: number) {
		if (!element) {
			return;
		}
		const rect = element.getBoundingClientRect();
		const file = getFile(x - rect.left);
		const rank = getRank(y - rect.top, file, isWhitePerspective);
		if (file < 0 || file > ranksPerFile.length || rank < 0 || rank > ranksPerFile[file]) {
			return;
		}
		return { file: file, rank: rank };
	}

	function onDropBoardPiece(from: Hex, x: number, y: number, piece: number) {
		hoveringHexagon = undefined;
		const hex = getHex(x, y);
		if (hex) {
			onDropPiece?.(from, hex, piece);
		}
	}

	function onDragBoardPiece(x: number, y: number) {
		hoveringHexagon = getHex(x, y);
	}

	const potentialMovesMap = $derived.by(() => {
		if (!potentialMoves) {
			return {};
		}
		const potentialMovesMap: Record<string, boolean> = {};
		for (const move of potentialMoves) {
			potentialMovesMap[move.file + "," + move.rank] = true;
		}
		return potentialMovesMap;
	});

	const actualBoard = $derived.by(() => board ? board : defaultBoard)
</script>

<div bind:this={element} class="board" style="width: {11 * hexHeight}px; height: {11 * hexHeight}px;">
	{#each actualBoard.file as piecesFile, file (file)}
		{#each piecesFile.pieces as piece, rank (rank)}
			{@const top = getTop(file, rank, isWhitePerspective)}
			{@const left = getLeft(file)}
			{@const bgIndex = (colorsOffset[file] + rank) % 3}
			{@const isMove = potentialMovesMap[file + "," + rank]}
			{@const isSelected = selectedHexagon?.file === file && selectedHexagon?.rank === rank}
			{@const isHovering = hoveringHexagon?.file === file && hoveringHexagon?.rank === rank}
			{@const isDraggable =
				draggable !== "none" &&
				(draggable === "anyone" ||
				(draggable === "turn" && isWhite(piece) && board?.isWhiteTurn) ||
				(draggable === "turn" && isBlack(piece) && !board?.isWhiteTurn))}
			<div
				class="hexagon"
				role="cell"
				tabindex="0"
				style:top="{top}px"
				style:left="{left}px"
				style:width="{hexWidth}px"
				style:height="{hexHeight}px"
				style:background-color={colors[bgIndex]}
			></div>
			<div
				class="hexagon"
				role="cell"
				tabindex="0"
				style:top="{top}px"
				style:left="{left}px"
				style:width="{hexWidth}px"
				style:height="{hexHeight}px"
				style:background-color={function(){
					if (isSelected) {
						return selectedColor
					} else if (isHovering) {
						return hoveringColor
					} else {
						return 'transparent';
					}
				}()}
				oncontextmenu={e => e.preventDefault()}
			>
				{#if piece !== pieces.empty}
					{#if isMove && !isHovering}
						<div class="move-circle" style:border-color={selectedColor}></div>
					{/if}
				{:else}
					{#if isMove && !isHovering}
						<div class="move-dot" style:background-color={selectedColor}></div>
					{/if}
				{/if}
			</div>
			{#if piece !== pieces.empty}
				<Piece
					{isSelected}
					isBgTransparent
					isDraggable={isDraggable}
					initialLeft={left}
					initialTop={top}
					piece={piece}
					onSelectPiece={() => onSelectBoardPiece({ file, rank })}
					onDeSelectPiece={() => onDeSelectBoardPiece({ file, rank })}
					onDragPiece={onDragBoardPiece}
					onDropPiece={(x, y, piece) => onDropBoardPiece({ file, rank }, x, y, piece)}
				/>
			{/if}
		{/each}
	{/each}
</div>

<style>
    .board {
        position: relative;
    }

    .hexagon {
        cursor: pointer;
		z-index: 1;
        overflow: hidden;
        position: absolute;
        aspect-ratio: 1 / cos(30deg);
        clip-path: polygon(50% -50%, 100% 50%, 50% 150%, 0 50%);
        display: flex;
        justify-content: center;
        user-select: none;
        -moz-user-select: none;
        -khtml-user-select: none;
        -webkit-user-select: none;
    }

    .move-dot {
        position: absolute;
        top: 50%;
        left: 50%;
        transform: translate(-50%, -50%);
		border-radius: 50%;
        height: 15px;
		width: 15px;
        cursor: pointer;
    }

    .move-circle {
        border: 5px solid;
        background-color: transparent;
        position: absolute;
        top: 50%;
        left: 50%;
        transform: translate(-50%, -50%);
        border-radius: 50%;
        height: 50px;
        width: 50px;
        cursor: pointer;
    }
</style>
