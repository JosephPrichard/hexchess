<script lang="ts">
	import type { ChessBoard, PieceMove } from '$lib/pb/messages';
	import Piece from './Piece.svelte';
	import type { Hex } from '$lib/api/models';
	import { defaultBoard, hexEq, isPieceWhite, iterBoard, makeMoveKey, pieces } from '$lib/service/chess';
	import {
		bgColors,
		darkGreen,
		findHex,
		findHexColor,
		getLeft,
		getTop,
		grey,
		viewportHeight,
		viewportWidth,
		hexHeight,
		hexPoints,
		hexWidth,
		lime,
		mediumPurple,
		white,
		findKeyedPieces,
		hexPathPoints,
		type PlacedPiece, ranksBeforeHalf, ranksAfterHalf, halfRank, topOffset, leftPlus, leftMinus
	} from '$lib/components/chess/render';
	import Fen from '$lib/components/chess/Fen.svelte';
	import { browser } from '$app/environment';
	import { CancelPromotion, type BadPromotionType, type MoveAction, type Promotion } from '$lib/components/chess/types';
	import { wasm } from '$lib/api/wasm';
	import { onMount } from 'svelte';

	export interface BoardProps {
		boardElement?: HTMLElement;
		board?: ChessBoard;
		fen?: string | boolean;
		draggable?: "turn" | "anyone" | "none";
		isWhitePerspective: boolean;
		hovering?: {piece: number, hex: Hex};
		selected?: Hex;
		promotion?: {from: Hex, to: Hex};
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
		boardElement = $bindable(),
		board, 
		fen,
		isWhitePerspective: isWhite,
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

	function onDragBoardPiece(piece: number, x: number, y: number) {
		const hex = findHex(boardElement, isWhite, x, y);
		if (!hex) return;
		hovering = {piece, hex};
	}

	function onDropBoardPiece(from: Hex, x: number, y: number, piece: number) {
		hovering = undefined;
		const hex = findHex(boardElement, isWhite, x, y);
		if (!hex) return
		if (draggable === "anyone" ||
			draggable === "turn" && isPieceWhite(piece) && board?.isWhiteTurn ||
			draggable === "turn" && !isPieceWhite(piece) && !board?.isWhiteTurn
		) {
			onDropPiece?.(from, hex);
		}
	}

	function onClickHexagon(e: MouseEvent) {
		e.preventDefault();
		const hex = findHex(boardElement, isWhite, e.clientX, e.clientY);
		if (!hex) return;
		if (selected) {
			onDropPiece?.(selected, hex);
		} else {
			onSetPiece?.(hex);
		}
	}

	const pieceMoves = $derived.by(() => {
		const pmm: Map<string, boolean> = new Map();
		if (potentialMoves) {
			for (const move of potentialMoves) pmm.set(makeMoveKey(move.file, move.rank), true)
		}
		return pmm;
	});

	const awaitingFen = $derived.by(async () => {
		if (typeof fen === "string") return Promise.resolve(fen)
		else if (fen && board && browser) return wasm.boardToFen(board);
	});

	const getIsMoveTarget = (hex: Hex) => pieceMoves.get(makeMoveKey(hex.file, hex.rank))

	function pickHexBgs(hex: Hex): string {
		const isPrevMove =
			hexEq({file: prevMove?.fromFile, rank: prevMove?.fromRank}, hex) ||
			hexEq({file: prevMove?.toFile, rank: prevMove?.toRank}, hex);
		const isNextMove = hexEq(nextMove?.from, hex) || hexEq(nextMove?.to, hex);

		const isMoveTarget = pieceMoves.get(makeMoveKey(hex.file, hex.rank));
		const isSelected = hexEq(selected, hex);

		let bgColor = "transparent";
		if (isNextMove) {
			bgColor = mediumPurple;
		} else if (isPrevMove) {
			bgColor = lime
		} else if (isSelected) {
			bgColor = darkGreen;
		}

		// the bg color for the hex, and the indicator for whether the hexagon is being targeted or not
		return bgColor;
	}

	function pickHexStrokes(hex: Hex): string {
		const isDraggable =
			(hovering && board?.isWhiteTurn === isPieceWhite(hovering?.piece) && draggable === "turn") ||
			draggable === "anyone";
		return hexEq(hovering?.hex, hex) && isDraggable ? white : "transparent";
	}

	const getNextPromotion = (hex: Hex) => hexEq(nextMove?.to, hex) ? nextMove?.promotion.piece : undefined;

	const getFileMarker = (file: number) => String.fromCharCode('a'.charCodeAt(0) + (isWhite ? file : (boardState.file.length - 1) - file));

	const [boardState, boardPieces] = $derived.by(() => {
		const boardState = board ?? defaultBoard;
		if (browser) {
			const boardPieces = findKeyedPieces(boardState, prevPieces)
			prevPieces = boardPieces;
			return [boardState, boardPieces];
		}
		return [boardState, []];
	});

	const flatPieces = $derived.by(() => {
		const result: PlacedPiece[] = [];
		iterBoard(boardState, (file, rank, piece) => result.push({ piece, file, rank }));
		return result;
	});

	const hexStyle = ({top, left}: {top: number, left: number}) => `
		top: ${top}px;
		left: ${left}px;
		width: ${hexWidth}px;
		height: ${hexHeight}px;`;

	onMount(() => {
		document.addEventListener('click', onClickHexagon);
		return () => {
			document.removeEventListener('click', onClickHexagon);
		}
	});
</script>

<div class="board-wrapper">
	<div bind:this={boardElement} class="board" style:width="{11 * hexHeight}px;" style:height="{11.67 * hexHeight}px;">
		{#each boardPieces as [index, p] (index)}
			{@const { file, rank } = p}
			{@const hex = {file, rank}}
			{@const top = getTop(file, rank, isWhite)}
			{@const left = getLeft(file, isWhite)}
			{@const isMoveTarget = pieceMoves.get(makeMoveKey(file, rank))}
			{@const isSelected = hexEq(selected, p)}
			{@const isPromoting = hexEq(promotion?.to, p)}
			<div class="piece-wrapper">
				<Piece
					isSelected={isSelected}
					isTransparent
					isAnnotatable
					isDraggable={true}
					isPromoting={isPromoting}
					isMoveTarget={isMoveTarget}
					nextPromotionPiece={getNextPromotion(hex)}
					initialLeft={left}
					initialTop={top}
					piece={p.piece}
					onSelectPiece={() => onSelectPiece?.(hex)}
					onDeSelectPiece={() => onDeSelectPiece?.(hex)}
					onDragPiece={onDragBoardPiece}
					onDropPiece={(x, y) => onDropBoardPiece(hex, x, y, p.piece)}
					onCompletePromotion={onCompletePromotion}
				/>
			</div>
		{/each}
		<svg class="svg-board"
			 width="{viewportWidth}px"
			 height="{viewportHeight}px"
			 viewBox="0 0 {viewportWidth} {viewportHeight}"
		>
			{#each flatPieces as {piece, file, rank}}
				{@const hex = {file, rank}}
				{@const top = getTop(file, rank, isWhite)}
				{@const left = getLeft(file, isWhite)}
				{@const bgIndex = findHexColor(hex)}
				{@const bgColor = pickHexBgs(hex)}
				{@const isMoveTarget = getIsMoveTarget(hex)}
				<path
					d={hexPathPoints(left, top, hexWidth, hexHeight)}
					fill={bgColors[bgIndex]}
					stroke={grey}
				/>
				<polygon points={hexPoints(left, top, hexWidth, hexHeight)} fill={bgColor} />
				{#if piece !== pieces.empty}
					{#if isMoveTarget}
						<circle
							cx={left + hexWidth / 2}
							cy={top + hexHeight / 2}
							r={hexWidth * 0.35}
							fill="none"
							stroke={darkGreen}
							stroke-width={hexWidth * 0.05}
						/>
					{/if}
				{:else}
					{#if isMoveTarget}
						<circle cx={left + hexWidth / 2} cy={top + hexHeight / 2} r={hexWidth * 0.1} fill={darkGreen} />
					{/if}
				{/if}
			{/each}
			{#each flatPieces as {file, rank}}
				{@const hex = {file, rank}}
				{@const top = getTop(file, rank, isWhite)}
				{@const left = getLeft(file, isWhite)}
				{@const strokeColor = pickHexStrokes(hex)}
				<polygon
					points={hexPoints(left, top, hexWidth, hexHeight)}
					fill="none"
					stroke={strokeColor}
					stroke-width={hexWidth * 0.04}
				/>
			{/each}
		</svg>
		{#each boardState.file as _, file}
			{@const top = getTop(file, -1, isWhite) + (isWhite ? 0 : hexHeight / 2.5)}
			{@const left = getLeft(file, isWhite)}
			<div class="hexagon-label" style={hexStyle({top, left})}>
				{getFileMarker(file)}
			</div>
		{/each}
		{#each [{ranks: ranksBeforeHalf, file: 0}, {ranks: ranksBeforeHalf, file: 10}] as {ranks, file}}
			{@const leftOffset = file <= halfRank ? (isWhite ? leftMinus : leftPlus) : (isWhite ? leftPlus : leftMinus)}
			{#each ranks as rank}
				{@const top = getTop(file, rank, isWhite) + topOffset(isWhite)}
				{@const left = getLeft(file, isWhite) + leftOffset}
				<div class="hexagon-label" style={hexStyle({top, left})}>
					{rank+1}
				</div>
			{/each}
		{/each}
		{#each ranksAfterHalf as rank}
			{@const leftOffset = isWhite ? leftMinus : leftPlus}
			{@const top = getTop(rank - halfRank, rank, isWhite) + topOffset(isWhite)}
			{@const left = getLeft(rank - halfRank, isWhite) + leftOffset}
			<div class="hexagon-label" style={hexStyle({top, left})}>
				{rank+1}
			</div>
		{/each}
		{#each ranksAfterHalf as rank}
			{@const leftOffset = isWhite ? leftPlus : leftMinus}
			{@const top = getTop(rank - halfRank, rank, isWhite) + topOffset(isWhite)}
			{@const left = getLeft(15 - rank, isWhite) + leftOffset}
			<div class="hexagon-label" style={hexStyle({top, left})}>
				{rank+1}
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
