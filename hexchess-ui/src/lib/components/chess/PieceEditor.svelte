<script lang="ts">
	import { hexHeight, hexWidth, findHex } from '$lib/components/chess/render';
	import Piece from './Piece.svelte';
	import type { Hex } from '$lib/api/models';
	import { type SelectEvent, selectEvents } from '$lib/globals';
	import { blackPieces, whitePieces } from '$lib/utils/chess.js';
	import CursorIcon from '$lib/components/icons/CursorIcon.svelte';
	import LargeTrashIcon from '$lib/components/icons/LargeTrashIcon.svelte';

	export interface PieceEditorProps {
		selectedPiece?: number;
		boardElement?: HTMLElement;
		hoveringHexagon?: Hex;
		isWhitePerspective?: boolean;
		onDropPiece?: (to: Hex) => void;
		isTrashSelector?: boolean;
	}

	let { selectedPiece = $bindable(), boardElement = $bindable(), hoveringHexagon = $bindable(),
		isWhitePerspective, onDropPiece, isTrashSelector = $bindable() }: PieceEditorProps = $props();

	function onSelectPiece(event: SelectEvent, piece: number) {
		if (isTrashSelector)
			return;
		switch (event) {
		case "SELECT":
			selectedPiece = piece;
			break;
		case "DESELECT":
			selectedPiece = undefined;
			break;
		}
	}

	function onDragEditorPiece(x: number, y: number) {
		hoveringHexagon = findHex(boardElement, isWhitePerspective, x, y);
	}

	function onDropEditorPiece(x: number, y: number) {
		hoveringHexagon = undefined;
		const hex = findHex(boardElement, isWhitePerspective, x, y);
		if (hex) {
			onDropPiece?.(hex);
		}
	}

	function onSelectTrash(isCursor: boolean = false) {
		isTrashSelector = !isCursor;
		if (isTrashSelector) {
			selectedPiece = undefined;
		}
	}
</script>

<div class="piece-editor">
	{#each [whitePieces, blackPieces] as panel, i}
		{@const isCursor = i === 0}
		<div class="piece-panel">
			{#each panel as piece}
				<div class="piece-tile-wrapper">
					<div
						role="cell"
						tabindex="0"
						class="piece-tile"
						style:width="{hexWidth}px"
						style:height="{hexHeight}px"
						class:selected-tile={selectedPiece === piece}
						onmousedown={(e) => onSelectPiece(selectEvents[e.button], piece)}
						class:disabled-piece-panel={isTrashSelector}
					>
						<Piece
							isDraggable={!isTrashSelector}
							piece={piece}
							initialLeft={-5}
							initialTop={0}
							onSelectHexagon={(event) => onSelectPiece(event, piece)}
							onDragPiece={onDragEditorPiece}
							onDropPiece={onDropEditorPiece}
						/>
					</div>
				</div>
			{/each}
			<div
				role="cell"
				tabindex="0"
				class="select-tile"
				style:width="{hexWidth}px"
				style:height="{hexHeight}px"
				class:red-select-tile={isTrashSelector && !isCursor}
				class:green-select-tile={!isTrashSelector && isCursor}
				onmousedown={() => onSelectTrash(isCursor)}
			>
				{#if isCursor}
					<CursorIcon/>
				{:else}
					<LargeTrashIcon/>
				{/if}
			</div>
		</div>
	{/each}
</div>

<style>
	.piece-editor {
		width: 100%;
		display: flex;
		flex-direction: row;
		gap: 25px;
        align-items: center;
        justify-content: center;
	}

	.selected-tile {
        background-color: #FFEB3B;
		border-radius: 3px;
	}

	.red-select-tile {
		background-color: #F44336;
	}

	.green-select-tile {
		background-color: #4CAF50;
	}

	.disabled-piece-panel {
		opacity: 0.4;
	}

	.piece-panel {
		border: 1px solid rgb(100,100,100);
        margin-top: 20px;
        margin-bottom: 20px;
        border-radius: 3px;
        background: rgb(64, 64, 64);
	}

	.piece-tile-wrapper {
		text-align: center;
		width: 100%;
	}

	.select-tile {
		cursor: pointer;
		display: flex;
        justify-content: center;
        align-items: center;
        border-radius: 3px;
        transition: background-color 0.15s ease-out;
	}

    .piece-tile {
		cursor: pointer;
		position: relative;
		margin: auto;
	}
</style>