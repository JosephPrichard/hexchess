<script lang="ts">
	import type { ChessBoard, PieceMove } from '$lib/api/messages';
	import Piece from './Piece.svelte';
	import type { Hex } from '$lib/api/model';
	import { defaultBoard, isBlack, isWhite, pieces, ranksPerFile } from '$lib/services/chess';
	import { getLeft, getTop, hexHeight, hexWidth, selectedColor, colors, colorsOffset, findHex, highlightedColor, hoveringColor } from '$lib/services/render';
	import Fen from '$lib/components/chess/Fen.svelte';

	export interface BoardProps {
		board?: ChessBoard;
		fen?: string;
		boardElement?: HTMLElement;
		draggable?: "turn" | "anyone" | "none";
		isWhitePerspective: boolean;
		hoveringHexagon?: Hex;
		selectedHexagon?: Hex;
		prevMove?: PieceMove;
		potentialMoves?: Hex[];
		onSelectPiece?: (hex: Hex) => void;
		onDeSelectPiece?: (hex: Hex) => void;
		onDropPiece?: (from: Hex, to: Hex) => void;
		onSetPiece?: (hex: Hex) => void;
	}

	let { board, fen, isWhitePerspective, boardElement = $bindable(), draggable,
		selectedHexagon, prevMove, hoveringHexagon = $bindable(), potentialMoves,
		onSelectPiece, onDeSelectPiece, onDropPiece, onSetPiece }: BoardProps = $props();

	function onDragBoardPiece(x: number, y: number) {
		hoveringHexagon = findHex(boardElement, isWhitePerspective, x, y);
	}

	function onDropBoardPiece(from: Hex, x: number, y: number, piece: number) {
		hoveringHexagon = undefined;
		const hex = findHex(boardElement, isWhitePerspective, x, y);
		if (!hex)
			return
		if (draggable === "anyone" ||
			draggable === "turn" && isWhite(piece) && board?.isWhiteTurn ||
			draggable === "turn" && isBlack(piece) && !board?.isWhiteTurn
		) {
			onDropPiece?.(from, hex);
		}
	}

	function onClickHexagon(file: number, rank: number) {
		if (selectedHexagon) {
			// the selected hexagon move takes priority
			onDropPiece?.(selectedHexagon, {file, rank});
		} else {
			// defaults to just "setting" a piece from an external editor, no-ops if that is not provided
			onSetPiece?.({file, rank});
		}
	}

	function makeMoveKey(file: number, rank: number) {
		return file + "," + rank;
	}

	const potentialMovesMap = $derived.by(() => {
		if (!potentialMoves) {
			return {};
		}
		const potentialMovesMap: Record<string, boolean> = {};
		for (const move of potentialMoves) {
			potentialMovesMap[makeMoveKey(move.file, move.rank)] = true;
		}
		return potentialMovesMap;
	});

	const boardState = $derived.by(() => board ?? defaultBoard);
</script>

<div class="board-wrapper">
	<div bind:this={boardElement} class="board" style:width="{11 * hexHeight}px;" style:height="{11.67 * hexHeight}px;">
		{#each boardState.file as piecesFile, file}
			{@const fileMarker = String.fromCharCode('a'.charCodeAt(0)
				+ (isWhitePerspective
					? file
					: (boardState.file.length - 1) - file))}
			{#each piecesFile.pieces as piece, rank}
				{@const rankMarker = rank + 1}
				{@const top = getTop(file, rank, isWhitePerspective)}
				{@const left = getLeft(file, isWhitePerspective)}
				{@const bgIndex = (colorsOffset[file] + rank) % 3}
				{@const isPrevMove =
					(prevMove?.fromFile === file && prevMove?.fromRank === rank) ||
					(prevMove?.toFile === file && prevMove?.toRank === rank)}
				{@const isMoveTarget = potentialMovesMap[makeMoveKey(file, rank)]}
				{@const isSelected = selectedHexagon?.file === file && selectedHexagon?.rank === rank}
				{@const isHovering = hoveringHexagon?.file === file && hoveringHexagon?.rank === rank}
				{@const isDraggable =
					draggable !== "none" &&
					(draggable === "anyone" || (draggable === "turn"))}
				{@const bgColor = function() {
					if (isPrevMove) {
						return highlightedColor
					} else if (isSelected) {
						return selectedColor
					} else if (isMoveTarget && isHovering) {
						return hoveringColor
					} else {
						return 'transparent';
					}
				}()}
				{#if file === 0 || (file <= 5 && rank === ranksPerFile[file]-1)}
					<div
						class="hexagon-label"
						style:top="{top + (isWhitePerspective ? -hexHeight / 4 : hexHeight / 1.6)}px"
						style:left="{getLeft(file, true) - hexWidth / 4.5}px"
					>
						{rankMarker}
					</div>
				{/if}
				{#if file === boardState.file.length-1 || (file >= 5 && rank === ranksPerFile[file]-1)}
					<div
						class="hexagon-label"
						style:top="{top + (isWhitePerspective ? -hexHeight / 4 : hexHeight / 1.6)}px"
						style:left="{getLeft(file, true) + hexWidth}px"
					>
						{rankMarker}
					</div>
				{/if}
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
					role="button"
					tabindex="0"
					style:top="{top}px"
					style:left="{left}px"
					style:width="{hexWidth}px"
					style:height="{hexHeight}px"
					style:background-color={bgColor}
					oncontextmenu={e => e.preventDefault()}
					onmousedown={() => onClickHexagon(file, rank)}
				>
					{#if piece !== pieces.empty}
						{#if isMoveTarget && !isHovering}
							<div
								class="move-circle"
								style:border-color={selectedColor}
								style:width="{hexWidth * 0.65}px"
								style:height="{hexWidth * 0.65}px"
							>
							</div>
						{/if}
					{:else}
						{#if isMoveTarget && !isHovering}
							<div class="move-dot" style:background-color={selectedColor}></div>
						{/if}
					{/if}
				</div>
				{#if piece !== pieces.empty}
					<Piece
						isSelected={isSelected}
						isTransparent
						isAnnotatable
						isDraggable={isDraggable}
						initialLeft={left}
						initialTop={top}
						piece={piece}
						onSelectHexagon={isMoveTarget ? () => onClickHexagon(file, rank) : undefined}
						onSelectPiece={() => onSelectPiece?.({ file, rank })}
						onDeSelectPiece={() => onDeSelectPiece?.({ file, rank })}
						onDragPiece={onDragBoardPiece}
						onDropPiece={(x, y) => onDropBoardPiece({ file, rank }, x, y, piece)}
					/>
				{/if}
			{/each}
			{@const top = isWhitePerspective ? getTop(file, -1, true) : getTop(file, -0.6, false)}
			{@const left = getLeft(file)}
			<div
				class="hexagon-label"
				style:top="{top}px"
				style:left="{left}px"
				style:width="{hexWidth}px"
				style:height="{hexHeight}px"
			>
				{fileMarker}
			</div>
		{/each}
	</div>
	{#if fen}
		<Fen fen={fen}/>
	{/if}
</div>

<style>
    .board {
        position: relative;
    }

	.board-wrapper {
		margin-top: 20px;
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
        -webkit-user-select: none;
    }

	.hexagon-label {
		padding-top: 5px;
		text-align: center;
		position: absolute;
		font-weight: bold;
		z-index: 100;
		color: rgb(150,150,150);
        user-select: none;
        -moz-user-select: none;
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
        cursor: pointer;
    }
</style>
