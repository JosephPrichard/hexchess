<script lang="ts">
	import ClipboardIcon from '$lib/icons/ClipboardIcon.svelte';
	import { getNotificationsContext } from '$lib/utils/context';
	import {ChessRenderer, defaultRenderArgs} from "$lib/components/chessRenderer";

	interface FENProps {
		fen: string;
		onChange?: (fen: string) => void;
	}

	const { addNotification } = getNotificationsContext();

	const { fen, onChange }: FENProps = $props();

	function onClickCopy() {
		navigator.clipboard.writeText(fen);
		addNotification({ type: 'string', isSuccess: true, message: 'Copied to clipboard!' })
	}

	function onChangeInput(e: Event) {
		const el = e.target as HTMLInputElement;
		onChange?.(el.value);
	}

	const render = new ChessRenderer(defaultRenderArgs);
</script>

<div style:width={render.getLeft(11.5) + "px"}>
	<div class="fen-wrapper">
		<label for="fen" class="fen-label">FEN</label>
		<input
			id="text"
			type="text"
			class="fen-input"
			value={fen}
			onkeydown={(e) => e.preventDefault()}
			onchange={onChangeInput}
			autocomplete="off"
			autocapitalize="off"
			spellcheck="false"
		/>
		<button class="icon" onclick={onClickCopy}>
			<ClipboardIcon/>
		</button>
	</div>
</div>

<style>
    .fen-wrapper {
        display: flex;
        align-items: center;
        gap: 8px;
		margin-top: 5px;
    }

	.fen-input {
        align-items: stretch;
		font-size: 14px;
		padding-left: 15px !important;
		padding-right: 15px !important;
		line-height: 38px;
	}

	.fen-label {
		font-size: 15px;
	}

    .fen-wrapper label {
        height: var(--label-height, auto);  /* y */
        display: flex;
        align-items: center
    }

    .fen-wrapper input {
        height: var(--input-height);
        padding: 0 8px;
        box-sizing: border-box;
    }

	.icon {
		background: none;
		border: none;
	}
</style>