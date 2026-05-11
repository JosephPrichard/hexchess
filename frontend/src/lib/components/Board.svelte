<script lang="ts">
	import type { ChessBoard, PieceMove } from '$lib/pb/messages';
	import Piece from './Piece.svelte';
	import type { Hex } from '$lib/api/models';
	import {chessService, defaultBoard, findKeyedPieces, pieces, type PlacedPiece} from '$lib/service/chess';
	import {defaultRenderArgs, ranksBeforeHalf, ranksAfterHalf, halfRank, type ChessRenderData, colors, ChessRenderer} from '$lib/components/chessRenderer';
	import Fen from '$lib/components/Fen.svelte';
	import { browser } from '$app/environment';
	import { type BadPromotionType, type MoveAction, type Promotion } from '$lib/components/types';
	import { wasm } from '$lib/api/wasm';
	import { onMount } from 'svelte';

	export interface BoardProps {
		boardElement?: HTMLElement;
		renderer?: ChessRenderData;
		showTileIndicators?: boolean;
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
		renderer: renderArgs,
		showTileIndicators,
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

	if (showTileIndicators === undefined) showTileIndicators = true;

	const render: ChessRenderer = new ChessRenderer(renderArgs ? renderArgs : defaultRenderArgs);

	let prevPieces: [number, PlacedPiece][] | undefined = undefined;

	function onDragBoardPiece(piece: number, x: number, y: number) {
		const hex = render.findHex(boardElement, isWhite, x, y);
		if (!hex) return;
		hovering = {piece, hex};
	}

	function onDropBoardPiece(from: Hex, x: number, y: number, piece: number) {
		hovering = undefined;
		const hex = render.findHex(boardElement, isWhite, x, y);
		if (!hex) return
		if (draggable === "anyone" ||
				draggable === "turn" && chessService.isPieceWhite(piece) && board?.isWhiteTurn ||
				draggable === "turn" && !chessService.isPieceWhite(piece) && !board?.isWhiteTurn
		) {
			onDropPiece?.(from, hex);
		}
	}

	function onClickHexagon(e: MouseEvent) {
		e.preventDefault();
		const hex = render.findHex(boardElement, isWhite, e.clientX, e.clientY);
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
			for (const move of potentialMoves) pmm.set(chessService.makeMoveKey(move.file, move.rank), true)
		}
		return pmm;
	});

	const awaitingFen = $derived.by(async () => {
		if (typeof fen === "string") return Promise.resolve(fen)
		else if (fen && board && browser) return wasm.boardToFen(board);
	});

	const getIsMoveTarget = (hex: Hex) => pieceMoves.get(chessService.makeMoveKey(hex.file, hex.rank))

	function pickHexBgs(hex: Hex): string {
		const isPrevMove =
				chessService.hexEq({file: prevMove?.fromFile, rank: prevMove?.fromRank}, hex) ||
				chessService.hexEq({file: prevMove?.toFile, rank: prevMove?.toRank}, hex);
		const isNextMove = chessService.hexEq(nextMove?.from, hex) || chessService.hexEq(nextMove?.to, hex);

		// const isMoveTarget = pieceMoves.get(makeMoveKey(hex.file, hex.rank));
		const isSelected = chessService.hexEq(selected, hex);

		let bgColor = "transparent";
		if (isNextMove) {
			bgColor = colors.mediumPurple;
		} else if (isPrevMove) {
			bgColor = colors.lime
		} else if (isSelected) {
			bgColor = colors.darkGreen;
		}

		// the bg color for the hex, and the indicator for whether the hexagon is being targeted or not
		return bgColor;
	}

	function pickHexStrokes(hex: Hex): string {
		const isDraggable =
				(hovering && board?.isWhiteTurn === chessService.isPieceWhite(hovering?.piece) && draggable === "turn") ||
				draggable === "anyone";
		return chessService.hexEq(hovering?.hex, hex) && isDraggable ? colors.white : "transparent";
	}

	const getNextPromotion = (hex: Hex) => chessService.hexEq(nextMove?.to, hex) ? nextMove?.promotion.piece : undefined;

	const getFileMarker = (file: number) => String.fromCharCode('a'.charCodeAt(0) + (isWhite ? file : (boardState.file.length - 1) - file));

	const [boardState, boardPieces] = $derived.by(() => {
		const boardState = board ?? defaultBoard;
		if (browser) {
			prevPieces = findKeyedPieces(boardState, prevPieces);
			return [boardState, prevPieces];
		}
		return [boardState, []];
	});

	const flatPieces = $derived.by(() => {
		const result: PlacedPiece[] = [];
		chessService.iterBoard(boardState, (file, rank, piece) => result.push({ piece, file, rank }));
		return result;
	});

	const hexStyle = ({top, left}: {top: number, left: number}) => `
		top: ${top}px;
		left: ${left}px;
		width: ${render.hexWidth}px;
		height: ${render.hexHeight}px;`;

	onMount(() => {
		document.addEventListener('click', onClickHexagon);
		return () => {
			document.removeEventListener('click', onClickHexagon);
		}
	});
</script>

<div>
	<div bind:this={boardElement} class="board" style:width="{11 * render.hexHeight}px;" style:height="{11.67 * render.hexHeight}px;">
		{#each boardPieces as [index, p] (index)}
			{@const { file, rank } = p}
			{@const hex = {file, rank}}
			{@const top = render.getTop(file, rank, isWhite)}
			{@const left = render.getLeft(file, isWhite)}
			{@const isMoveTarget = pieceMoves.get(chessService.makeMoveKey(file, rank))}
			{@const isSelected = chessService.hexEq(selected, p)}
			{@const isPromoting = chessService.hexEq(promotion?.to, p)}
			<div class="piece-wrapper">
				<Piece
					renderer={renderArgs}
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
			 width="{render.viewportWidth}px"
			 height="{render.viewportHeight}px"
			 viewBox="0 0 {render.viewportWidth} {render.viewportHeight}"
		>
			{#each flatPieces as {piece, file, rank}}
				{@const hex = {file, rank}}
				{@const top = render.getTop(file, rank, isWhite)}
				{@const left = render.getLeft(file, isWhite)}
				{@const bgIndex = render.findHexColor(hex)}
				{@const bgColor = pickHexBgs(hex)}
				{@const isMoveTarget = getIsMoveTarget(hex)}
				<path
					d={render.hexPathPoints(left, top)}
					fill={colors.bgColors[bgIndex]}
					stroke={colors.grey}
				/>
				<polygon points={render.hexPoints(left, top)} fill={bgColor} />
				{#if piece !== pieces.empty}
					{#if isMoveTarget}
						<circle
							cx={left + render.hexWidth / 2}
							cy={top + render.hexHeight / 2}
							r={render.hexWidth * 0.35}
							fill="none"
							stroke={colors.darkGreen}
							stroke-width={render.hexWidth * 0.05}
						/>
					{/if}
				{:else}
					{#if isMoveTarget}
						<circle
							cx={left + render.hexWidth / 2}
							cy={top + render.hexHeight / 2}
							r={render.hexWidth * 0.1}
							fill={colors.darkGreen}
						/>
					{/if}
				{/if}
			{/each}
			{#each flatPieces as {file, rank}}
				{@const hex = {file, rank}}
				{@const top = render.getTop(file, rank, isWhite)}
				{@const left = render.getLeft(file, isWhite)}
				{@const strokeColor = pickHexStrokes(hex)}
				<polygon
						points={render.hexPoints(left, top)}
						fill="none"
						stroke={strokeColor}
						stroke-width={render.hexWidth * 0.04}
				/>
			{/each}
		</svg>
		{#if showTileIndicators}
			{#each boardState.file as _, file}
				{@const top = render.getTop(file, -1, isWhite) + (isWhite ? 0 : render.hexHeight / 2.5)}
				{@const left = render.getLeft(file, isWhite)}
				<div class="hexagon-label" style={hexStyle({top, left})}>
					{getFileMarker(file)}
				</div>
			{/each}
			{#each [{ranks: ranksBeforeHalf, file: 0}, {ranks: ranksBeforeHalf, file: 10}] as {ranks, file}}
				{@const leftOffset = file <= halfRank ? (isWhite ? render.leftMinus : render.leftPlus) : (isWhite ? render.leftPlus : render.leftMinus)}
				{#each ranks as rank}
					{@const top = render.getTop(file, rank, isWhite) + render.topOffset(isWhite)}
					{@const left = render.getLeft(file, isWhite) + leftOffset}
					<div class="hexagon-label" style={hexStyle({top, left})}>
						{rank+1}
					</div>
				{/each}
			{/each}
			{#each ranksAfterHalf as rank}
				{@const leftOffset = isWhite ? render.leftMinus : render.leftPlus}
				{@const top = render.getTop(rank - halfRank, rank, isWhite) + render.topOffset(isWhite)}
				{@const left = render.getLeft(rank - halfRank, isWhite) + leftOffset}
				<div class="hexagon-label" style={hexStyle({top, left})}>
					{rank+1}
				</div>
			{/each}
			{#each ranksAfterHalf as rank}
				{@const leftOffset = isWhite ? render.leftPlus : render.leftMinus}
				{@const top = render.getTop(rank - halfRank, rank, isWhite) + render.topOffset(isWhite)}
				{@const left = render.getLeft(15 - rank, isWhite) + leftOffset}
				<div class="hexagon-label" style={hexStyle({top, left})}>
					{rank+1}
				</div>
			{/each}
		{/if}
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
		/*margin-top: 20px;*/
		/*margin-left: 20px;*/
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
