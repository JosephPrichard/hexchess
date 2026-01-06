<script lang="ts">
	import { type ColorSelect, type GameMode, GameModeNameMap } from '$lib/api/models';
	import Dropdown from '$lib/components/util/Dropdown.svelte';

	interface Props {
		title: string;
		show: boolean;
		onSubmit: (mode: GameMode, color: ColorSelect) => void;
		fen?: string;
	}

	let { title, show = $bindable(), onSubmit, fen = $bindable() }: Props = $props();

	let color: ColorSelect = $state('RANDOM');
	let mode: GameMode = $state('CORRESPONDENCE_1');

	function onClickClose(e: MouseEvent) {
		e.preventDefault();
		show = false;
	}

	function onSubmitForm(e: MouseEvent) {
		e.preventDefault();
		onSubmit(mode, color);
	}

	function onClickColor(e: MouseEvent, newColor: ColorSelect) {
		e.preventDefault();
		color = newColor;
	}

	const options = Object.entries(GameModeNameMap).map(([key, value]) => ({label: value, value: key as GameMode}));
</script>

<div class="modal-overlay" id="modal-overlay" style:display={show ? '' : 'none'}></div>
<div class="absolute-center">
	<div class="modal panel" id="create-modal" style:display={show ? '' : 'none'}>
		<form class="form-wrapper" id="create-game-form">
			<div class="text-md modal-panel">
				{title}
			</div>
			<button class="modal-x" onclick={onClickClose}> &#10006; </button>
			<div class="modal-panel">
				<div class="text-xsm" style="margin-bottom: 5px"> Game Mode </div>
				<div class="mode-input">
					<Dropdown
						options={options}
						selected="TIMED_1+0"
						onChange={value => mode = value}
					/>
				</div>
			</div>
			<div class="modal-panel">
				<button class="piece-color invisible-button" style:display="inline-block" tabindex="-1" onclick={(e) => onClickColor(e, 'BLACK')} >
					<img alt="Black" class="color-piece-image" class:selected-color-piece-image={color === 'BLACK'} src="/pieces/black-king.png" />
				</button>
				<button class="piece-color invisible-button" style:display="inline-block" tabindex="-1" onclick={(e) => onClickColor(e, 'RANDOM')}>
					<img alt="Random" class="color-piece-image" class:selected-color-piece-image={color === 'RANDOM'} src="/pieces/half-king.png" />
				</button>
				<button class="piece-color invisible-button" style:display="inline-block" tabindex="-1" onclick={(e) => onClickColor(e, 'WHITE')}>
					<img alt="White" class="color-piece-image" class:selected-color-piece-image={color === 'WHITE'} src="/pieces/white-king.png" />
				</button>
			</div>
			{#if fen}
				<div class="fen-input">
					<input bind:value={fen}/>
				</div>
			{/if}
			<button class="button button-grey" onclick={onSubmitForm} type="submit"> Create! </button>
		</form>
	</div>
</div>

<style>
    .modal-x {
        position: absolute;
        top: 20px;
        right: 20px;
        height: 25px;
        cursor: pointer;
        background: rgb(210, 4, 45);
        border: none;
        border-radius: 2px;
    }

    .modal-x:hover {
        background: rgb(250, 44, 85);
    }

    .modal-panel {
        margin-bottom: 25px;
    }

    .color-piece-image {
        width: 65px;
        height: 65px;
        padding: 5px;
        border-radius: 5px;
        cursor: pointer;
        display: inline-block;
        background-color: rgb(55, 55, 55);
        box-shadow: rgba(0, 0, 0, 0.24) 0 1px 3px;
    }

    .selected-color-piece-image {
        background-color: rgb(99, 99, 99);
    }

	.piece-color {
		cursor: pointer;
	}

	.mode-input {
		height: 30px;
		border-radius: 5px;
	}

	.fen-input {
		margin-bottom: 25px;
	}
</style>