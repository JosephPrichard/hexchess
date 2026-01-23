<script lang="ts">
	import type { ChessBoard, PieceMove } from '$lib/pb/messages';
	import Piece from './Piece.svelte';
	import type { Hex } from '$lib/api/models';
	import { defaultBoard, findKeyedPieces, hexEq, pieces, type PlacedPiece, ranksPerFile } from '$lib/utils/chess.js';
	import { bgColors, darkGreen, findHex, findHexColor, getLeft, getTop, hexHeight, hexWidth, lightGreen, lime, mediumPurple } from '$lib/components/chess/render';
	import Fen from '$lib/components/chess/Fen.svelte';
	import { browser } from '$app/environment';
	import { boardToFenWasm } from '$lib/api/wasm';
	import type { BadPromotionType, MoveAction, Promotion } from '$lib/components/chess/types';
	import type { PromotionMove } from '$lib/state/game.svelte';

	export interface BoardProps {
		board?: ChessBoard;
		fen?: string | boolean;
		boardElement?: HTMLElement;
		draggable?: "turn" | "anyone" | "none";
		isWhitePerspective: boolean;
		hovering?: Hex;
		selected?: Hex;
		promotion?: PromotionMove;
		prevMove?: PieceMove;
		nextMove?: MoveAction;
		potentialMoves?: Hex[];
		onSelectPiece?: (hex: Hex) => void;
		onDeSelectPiece?: (hex: Hex) => void;
		onDropPiece?: (from: Hex, to: Hex) => void;
		onSetPiece?: (hex: Hex) => void;
		onCompletePromotion?: (promotedPiece: Promotion | BadPromotionType) => void;
		onChangeFen?: (fen: string) => void;
	}

	let { 
		board, 
		fen,
		isWhitePerspective, 
		boardElement = $bindable(),
		draggable,
		selected,
		hovering = $bindable(),
		promotion,
		potentialMoves,
		prevMove,
		nextMove,
		onSelectPiece,
		onDeSelectPiece,
		onDropPiece,
		onSetPiece,
		onCompletePromotion,
		onChangeFen
	}: BoardProps = $props();

	let prevPieces: [number, PlacedPiece][] | undefined = undefined;

	function onDragBoardPiece(x: number, y: number) {
		hovering = findHex(boardElement, isWhitePerspective, x, y);
	}

	function onDropBoardPiece(from: Hex, x: number, y: number, piece: number) {
		hovering = undefined;
		const hex = findHex(boardElement, isWhitePerspective, x, y);
		if (!hex) return
		// if (draggable === "anyone" ||
		// 	draggable === "turn" && isPieceWhite(piece) && board?.isWhiteTurn ||
		// 	draggable === "turn" && isPieceBlack(piece) && !board?.isWhiteTurn
		// ) {
			onDropPiece?.(from, hex);
		// }
	}

	function onClickHexagon(file: number, rank: number) {
		if (selected) {
			onDropPiece?.(selected, {file, rank});
		} else {
			onSetPiece?.({file, rank});
		}
	}

	function makeMoveKey(file: number, rank: number) {
		return file + "," + rank;
	}

	const pieceMoves = $derived.by(() => {
		const pmm: Map<string, boolean> = new Map();
		if (potentialMoves) {
			for (const move of potentialMoves) {
				pmm.set(makeMoveKey(move.file, move.rank), true);
			}
		}
		return pmm;
	});

	const [boardState, boardPieces] = $derived.by(() => {
		const boardState = board ?? defaultBoard;
		if (browser) {
			const boardPieces = findKeyedPieces(boardState, prevPieces)
			prevPieces = boardPieces;
			// console.log("pieceEntries", boardPieces);
			return [boardState, boardPieces];
		}
		return [boardState, []];
	});

	const awaitingFen = $derived.by(async () => {
		if (typeof fen === "string") {
			return Promise.resolve(fen);
		} else if (fen && board && browser) {
			return boardToFenWasm(board);
		}
	});

	function pickHexOverlays(hex: Hex): [string, boolean | undefined] {
		const isPrevMove =
			hexEq({file: prevMove?.fromFile, rank: prevMove?.fromRank}, hex) ||
			hexEq({file: prevMove?.toFile, rank: prevMove?.toRank}, hex);
		const isNextMove = hexEq(nextMove?.from, hex) || hexEq(nextMove?.to, hex);

		const isMoveTarget = pieceMoves.get(makeMoveKey(hex.file, hex.rank));
		const isSelected = hexEq(selected, hex);
		const isHovering = hexEq(hovering, hex);
		let color = "";
		if (isNextMove) {
			color = mediumPurple;
		} else if (isPrevMove) {
			color = lime
		} else if (isSelected) {
			color = darkGreen
		} else if (isMoveTarget && isHovering) {
			color = lightGreen
		} else {
			color = 'transparent';
		}
		// the bg color for the hex, and the indicator for whether the hexagon is being targeted or not
		return [color, isMoveTarget && !isHovering];
	}

	function getNextPromotion(hex: Hex) {
		const isNextMoveTarget = hexEq(nextMove?.to, hex);
		return isNextMoveTarget ? nextMove?.promotion.piece : undefined;
	}

	function getFileMarker(file: number) {
		return String.fromCharCode('a'.charCodeAt(0)
			+ (isWhitePerspective
				? file
				: (boardState.file.length - 1) - file))
	}
</script>

<div class="board-wrapper">
	<div bind:this={boardElement} class="board" style:width="{11 * hexHeight}px;" style:height="{11.67 * hexHeight}px;">
		{#each boardPieces as [index, p] (index)}
			{@const { file, rank } = p}
			{@const hex = {file, rank}}
			{@const top = getTop(file, rank, isWhitePerspective)}
			{@const left = getLeft(file, isWhitePerspective)}
			{@const isMoveTarget = pieceMoves.get(makeMoveKey(file, rank))}
			{@const isSelected = hexEq(selected, p)}
			{@const isPromoting = hexEq(promotion?.to, p)}
			{@const isDraggable = draggable !== "none" && (draggable === "anyone" || (draggable === "turn"))}
			<div class="piece-wrapper">
				<Piece
					isSelected={isSelected}
					isTransparent
					isAnnotatable
					isDraggable={isDraggable}
					isPromoting={isPromoting}
					nextPromotion={getNextPromotion(hex)}
					initialLeft={left}
					initialTop={top}
					piece={p.piece}
					onSelectHexagon={isMoveTarget ? () => onClickHexagon(file, rank) : undefined}
					onSelectPiece={() => onSelectPiece?.(hex)}
					onDeSelectPiece={() => onDeSelectPiece?.(hex)}
					onDragPiece={onDragBoardPiece}
					onDropPiece={(x, y) => onDropBoardPiece(hex, x, y, p.piece)}
					onCompletePromotion={onCompletePromotion}
				/>
			</div>
		{/each}
		{#each boardState.file as piecesFile, file}
			{@const fileMarker = getFileMarker(file)}
			{#each piecesFile.pieces as piece, rank}
				{@const hex = {file, rank}}
				{@const rankMarker = rank + 1}
				{@const top = getTop(file, rank, isWhitePerspective)}
				{@const left = getLeft(file, isWhitePerspective)}
				{@const bgIndex = findHexColor(hex)}
				{@const [bgColor, isMoveTarget] = pickHexOverlays(hex)}
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
					role="button"
					tabindex="0"
					style:top="{top}px"
					style:left="{left}px"
					oncontextmenu={e => e.preventDefault()}
					onmousedown={() => onClickHexagon(file, rank)}
				>
					<svg width="{hexWidth}px" height="{hexHeight}px" viewBox="0 0 200 173">
						<polygon
							points="50,0 150,0 200,86.6 150,173 50,173 0,86.6"
							fill={bgColors[bgIndex]}
							stroke="rgba(120, 90, 60, 0.5)"
						/>
						<polygon points="50,0 150,0 200,86.6 150,173 50,173 0,86.6" fill={bgColor} />
						{#if piece !== pieces.empty}
							{#if isMoveTarget}
								<circle cx="100" cy="86.5" r="75" fill="none" stroke={darkGreen} stroke-width="10" />
							{/if}
						{:else}
							{#if isMoveTarget}
								<circle cx="100" cy="86.5" r="20" fill={darkGreen} stroke-width="2" />
							{/if}
						{/if}
					</svg>
				</div>
			{/each}
			{@const top = isWhitePerspective ? getTop(file, -1, true) : getTop(file, -0.6, false)}
			{@const left = getLeft(file)}
			<div class="hexagon-label" style:top="{top}px" style:left="{left}px" style:width="{hexWidth}px" style:height="{hexHeight}px">
				{fileMarker}
			</div>
		{/each}
	</div>
	{#await awaitingFen then fen}
		{#if fen}
			<Fen fen={fen} onChange={onChangeFen}/>
		{/if}
	{/await}
</div>

<style>
    .board {
        position: relative;
		margin-top: 20px;
		margin-left: 20px;
    }

	.board-wrapper {
		margin-top: 20px;
	}

	.hexagon {
		position: absolute;
	}

    .hexagon-label {
		padding-top: 5px;
		text-align: center;
		position: absolute;
		font-weight: bold;
        z-index: 1;
		color: rgb(150,150,150);
        user-select: none;
        -moz-user-select: none;
        -webkit-user-select: none;
	}
</style>
