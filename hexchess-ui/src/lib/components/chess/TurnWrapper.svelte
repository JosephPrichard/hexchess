<script lang="ts">
	import type { Snippet } from 'svelte';

	export interface TurnWrapperProps {
		isWhitePerspective?: boolean,
		isWhiteTurn?: boolean
		isEdged?: boolean
		children?: Snippet
	}
	const { isWhitePerspective, isWhiteTurn, isEdged, children }: TurnWrapperProps = $props();
</script>

<div class="side-table-header" class:no-rounded={isEdged}>
	<div class="turn-circle" class:turn-circle-green={Boolean(isWhiteTurn) !== isWhitePerspective}></div>
	{isWhitePerspective ? "Black's Turn" : "White's Turn"}
</div>
{#if children}
	{@render children()}
{/if}
<div class="side-table-header-bottom side-table-header-bottom-shadow" class:no-rounded={isEdged}>
	<div class="turn-circle" class:turn-circle-green={Boolean(isWhiteTurn) === isWhitePerspective}></div>
	{isWhitePerspective ? "White's Turn" : "Black's Turn"}
</div>

<style>
	.no-rounded {
		border-radius: 0 !important;
	}
</style>