<script lang="ts">
    import type { ColorSelect, TimeControl } from '$lib/models';

    interface Props {
        title: string;
        show: boolean;
        onSubmit: (timeControl: TimeControl, color: ColorSelect) => void;
        onClose: () => void;
    }

    const { title, show, onSubmit, onClose }: Props = $props();

    let color: ColorSelect = $state('RANDOM');
    let timeControl: TimeControl = $state('UNLIMITED');
</script>

<div class="modal-overlay" id="modal-overlay" style:display={show ? '' : 'none'}></div>
<div class="absolute-center">
    <div class="modal panel invisible" id="create-modal" style:display={show ? '' : 'none'}>
        <form class="form-wrapper" id="create-game-form">
            <div class="text-md" style="margin-bottom: 25px">
                {title}
            </div>
            <button class="modal-x" onclick={onClose}> &#10006; </button>
            <div style="margin-bottom: 25px">
                <label class="text-xsm" for="time-control"> Time control </label>
                <select bind:value={timeControl} name="time-control">
                    <option value="UNLIMITED"> Unlimited</option>
                    <option value="REAL_TIME"> Real Time</option>
                    <option value="CORRESPONDENCE"> Correspondence</option>
                </select>
            </div>
            <div id="selected-color" style="margin-bottom: 25px">
                <div role="button" tabindex="-1" onkeydown={() => (color = 'BLACK')}>
                    <img alt="Black" class="color-piece-image" class:selected-color-piece-image={color === 'BLACK'} src="/pieces/black-king.png" />
                </div>
                <div role="button" tabindex="-1" onkeydown={() => (color = 'RANDOM')}>
                    <img alt="Random" class="color-piece-image" class:selected-color-piece-image={color === 'RANDOM'} src="/pieces/half-king.png" />
                </div>
                <div role="button" tabindex="-1" onkeydown={() => (color = 'WHITE')}>
                    <img alt="White" class="color-piece-image" class:selected-color-piece-image={color === 'WHITE'} src="/pieces/white-king.png" />
                </div>
            </div>
            <button class="button button-grey" onclick={() => onSubmit(timeControl, color)} type="submit"> Create! </button>
        </form>
    </div>
</div>
