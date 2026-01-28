<script lang="ts">
	import { findHex, hexHeight, hexWidth } from '$lib/components/chess/render';
	import Piece from './Piece.svelte';
	import type { Hex } from '$lib/api/models';
	import { type SelectEvent, selectEvents } from '$lib/globals';
	import { blackPieces, whitePieces } from '$lib/service/chess';
	import CursorIcon from '$lib/components/icons/CursorIcon.svelte';
	import LargeTrashIcon from '$lib/components/icons/LargeTrashIcon.svelte';

	export interface PieceEditorProps {
		selectedPiece?: number;
		boardElement?: HTMLElement;
		hovering?: {hex: Hex, piece: number};
		isWhitePerspective?: boolean;
		onDropPiece?: (to: Hex) => void;
		isTrashSelector?: boolean;
	}

	let { selectedPiece = $bindable(), boardElement = $bindable(), hovering = $bindable(),
		isWhitePerspective, onDropPiece, isTrashSelector = $bindable() }: PieceEditorProps = $props();

	function onSelectPiece(event: SelectEvent, piece: number) {
		if (isTrashSelector) return;
		if (event === "SELECT") {
			selectedPiece = piece;
		} else if (event === "DESELECT" && hovering?.piece !== piece) {
			selectedPiece = undefined;
		}
	}

	function onDragEditorPiece(piece: number, x: number, y: number) {
		const hex = findHex(boardElement, isWhitePerspective, x, y);
		if (!hex) return;
		hovering = {piece, hex};
	}

	function onDropEditorPiece(x: number, y: number) {
		hovering = undefined;
		const hex = findHex(boardElement, isWhitePerspective, x, y);
		if (hex) {
			onDropPiece?.(hex);
		}
	}

	function onSelectTrash(isCursor: boolean = false) {
		isTrashSelector = !isCursor;
		if (isTrashSelector) selectedPiece = undefined;
	}
</script>

<div class="piece-editor">
	<div class="piece-panel">
		{#each [whitePieces, blackPieces] as panel}
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
							onDragPiece={(x, y) => onDragEditorPiece(piece, x, y)}
							onDropPiece={onDropEditorPiece}
						/>
					</div>
				</div>
			{/each}
		{/each}
	</div>
	<div class="piece-panel select-tile-wrapper">
		{#each ["CURSOR", "TRASH"] as selector, i}
			{@const isCursor = selector === "CURSOR"}
			<div>
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
</div>

<style>
	.piece-editor {
		width: 100%;
		display: flex;
		flex-direction: column;
		margin-top: 20px;
        gap: 20px;
        justify-content: space-between;
		align-items: center;
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
        box-shadow: 0 2px 4px rgba(0, 0, 0, 0.35);
        /*border: 1px solid rgb(100, 100, 100);*/
        border-radius: 3px;
        background: rgb(43, 43, 43);
		display: flex;
        flex-direction: row;
        flex-wrap: wrap;
	}

	.piece-tile-wrapper {
        width: 67px; /* todo: make this not hardcoded. */
		z-index: 1;
		text-align: center;
	}

    .piece-tile {
        border-radius: 2px;
        border: 1px solid rgb(100, 100, 100);
        cursor: pointer;
        position: relative;
        margin: 0;
		text-align: center;
    }

	.select-tile {
		cursor: pointer;
		display: flex;
        justify-content: center;
        align-items: center;
        border-radius: 3px;
        transition: background-color 0.15s ease-out;
	}

	.select-tile-wrapper {
        border: 1px solid rgb(100, 100, 100);
	}
</style>