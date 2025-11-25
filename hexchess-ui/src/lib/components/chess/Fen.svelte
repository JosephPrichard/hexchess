<script lang="ts">
	import { getLeft } from '$lib/components/chess/render';
	import ClipboardIcon from '$lib/components/icons/ClipboardIcon.svelte';
	import { getNotificationsContext } from '$lib/utils/context';

	interface FENProps {
		fen: string
	}

	const { addNotification } = getNotificationsContext();

	const { fen }: FENProps = $props();

	function onClickCopy() {
		navigator.clipboard.writeText(fen);
		addNotification({ type: 'string', isSuccess: true, message: 'Copied to clipboard!' })
	}
</script>

<div style:width={getLeft(11.5) + "px"}>
	<div class="fen-wrapper">
		<label for="fen" class="fen-label">FEN</label>
		<input id="text" type="text" class="fen-input" value={fen} disabled/>
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