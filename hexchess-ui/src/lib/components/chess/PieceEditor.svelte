<script lang="ts">
	import { hexHeight, hexWidth, findHex } from '$lib/services/render';
	import Piece from './Piece.svelte';
	import type { Hex } from '$lib/api/model';
	import { type SelectEvent, selectEvents } from '$lib/globals';
	import { blackPieces, whitePieces } from '$lib/services/chess';

	export interface PieceEditorProps {
		selectedPiece?: number;
		isDisabled?: boolean;
		boardElement?: HTMLElement;
		hoveringHexagon?: Hex;
		isWhitePerspective?: boolean;
		onDropPiece?: (to: Hex) => void;
	}

	let { selectedPiece = $bindable(), isDisabled, boardElement = $bindable(),
		hoveringHexagon = $bindable(), isWhitePerspective, onDropPiece }: PieceEditorProps = $props();

	function onSelectPiece(event: SelectEvent, piece: number) {
		if (isDisabled)
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
</script>

<div class="piece-editor">
	{#each [whitePieces, blackPieces] as panel}
		<div class="piece-panel" class:disabled-piece-panel={isDisabled}>
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
					>
						<Piece
							isDraggable={!isDisabled}
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
		</div>
	{/each}
</div>

<style>
	.piece-editor {
		width: 100%;
		margin-top: 20px;
		margin-bottom: 20px;
		padding: 20px;
		display: flex;
		flex-direction: row;
        border-radius: 3px;
        background: rgb(43, 43, 43);
	}

	.selected-tile {
        background-color: #FFEB3B;
		border-radius: 3px;
	}

	.disabled-piece-panel {
		opacity: 0.4;
	}

	.piece-panel {
        flex: 0.5;
	}

	.piece-tile-wrapper {
		text-align: center;
		width: 100%;
	}

    .piece-tile {
		cursor: pointer;
		position: relative;
		margin: auto;
	}
</style>