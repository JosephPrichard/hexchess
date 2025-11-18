<script lang="ts">
	import type { ColorSelect, TimeControl } from '../../api/model';

	interface Props {
		title: string;
		show: boolean;
		onSubmit: (timeControl: TimeControl, color: ColorSelect) => void;
		onClose: () => void;
	}

	const { title, show, onSubmit, onClose }: Props = $props();

	let color: ColorSelect = $state('RANDOM');
	let timeControl: TimeControl = $state('UNLIMITED');

	function onClickClose(e: MouseEvent) {
		e.preventDefault();
		onClose();
	}

	function onSubmitForm(e: MouseEvent) {
		e.preventDefault();
		onSubmit(timeControl, color);
	}

	function onClickColor(e: MouseEvent, newColor: ColorSelect) {
		e.preventDefault();
		color = newColor;
	}
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
				<label class="text-xsm" for="time-control"> Time control </label>
				<select bind:value={timeControl} name="time-control" class="time-control-input">
					<option value="UNLIMITED"> Unlimited</option>
					<option value="REAL_TIME"> Real Time</option>
					<option value="CORRESPONDENCE"> Correspondence</option>
				</select>
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
			<button class="button button-grey" onclick={onSubmitForm} type="submit"> Create! </button>
		</form>
	</div>
</div>

<style>
	.modal-panel {
        margin-bottom: 25px;
	}

    .modal-overlay {
        position: fixed;
        top: 0;
        left: 0;
        width: 100%;
        height: 100%;
        background-color: rgba(0, 0, 0, 0.35);
        z-index: 100;
    }

    .modal {
        z-index: 1000;
        width: 400px;
    }

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

	.time-control-input {
		height: 30px;
		border-radius: 5px;
	}
</style>