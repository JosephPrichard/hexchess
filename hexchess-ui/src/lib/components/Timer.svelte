<script lang="ts">
	import { formatTimer } from '$lib/utils/format';

	const dangerTimerThreshold = 15000;

	const { value, size, color }: { value?: number | bigint, size: "sm" | "lg", color?: "white" | "black" } = $props();

	const fontSize = $derived.by(() => {
		switch (size) {
		case "sm": return 17;
		case "lg": return 52;
		}
	});
</script>

{#if value !== undefined}
	<div class="timer"
		 style:font-size="{fontSize}px"
		 class:timer-warn={!color && value < dangerTimerThreshold}
		 class:timer-white={color === "white"}
		 class:timer-black={color === "black"}
	>
		<div class="timer-wrapper">
<!--			<ClockIcon color={color === "white" ? "black" : "white"}/>-->
			<div class="timer-text">
				{formatTimer(Number(value))}
			</div>
		</div>
	</div>
{/if}

<style>
	.timer-text {
        flex: 1;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
		/*max-width: 75px;*/
	}

	.timer-wrapper {
		display: flex;
		align-items: center;
		gap: 5px;
	}

    .timer {
        font-weight: bold;
        background-color: rgba(42, 42, 42);
        color: rgb(160, 160, 160);
        font-family: monospace;
        letter-spacing: 0.05rem;
        text-align: center;
        padding: 15px;
    }

	.timer-white {
        color: black;
        background-color: rgba(255, 255, 255, 0.75);
        border-radius: 5px;
	}

	.timer-black {
        color: white;
        background-color: rgba(0, 0, 0, 0.4);
        border-radius: 5px;
	}

    .timer-warn {
        color: rgba(255, 10, 10, 0.6);
    }
</style>