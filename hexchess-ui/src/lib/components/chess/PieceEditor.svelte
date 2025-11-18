<script lang="ts">
	import { hexHeight, hexWidth, findHex } from '../../services/render';
	import Piece from './Piece.svelte';
	import type { Hex } from '../../api/model';
	import { type SelectEvent, selectEvents } from '../../globals';
	import { blackPieces, whitePieces } from '../../services/chess';

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

{#each [whitePieces, blackPieces] as panel}
	<div class="piece-panel" class:disabled-piece-panel={isDisabled}>
		{#each panel as piece}
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
					initialLeft={0}
					initialTop={0}
					onSelectHexagon={(event) => onSelectPiece(event, piece)}
					onDragPiece={onDragEditorPiece}
					onDropPiece={onDropEditorPiece}
				/>
			</div>
		{/each}
	</div>
{/each}

<style>
	.selected-tile {
        background-color: rgb(30, 144, 255, 0.2);
	}

	.disabled-piece-panel {
		opacity: 0.4;
	}

	.piece-panel {
		border-radius: 3px;
		background-color: rgb(70, 70, 70);
        box-shadow:  2px 3px 5px rgba(0, 0, 0, .3) inset;
		position: relative;
	}

    .piece-tile {
		cursor: pointer;
		position: relative;
		padding: 1px;
	}
</style>