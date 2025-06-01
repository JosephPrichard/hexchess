<script lang="ts">
	import { formatElo, getResultClasses } from '$lib/format.js';
	import type { Replay } from '$lib/models';

	const { replay }: { replay: Replay } = $props();
	const [whiteClass, blackClass] = $derived(getResultClasses(replay.result));
</script>

<div class="side-table-header-elem">
	<a href="/players/{replay.whiteId}" class="text-ul">
		<b>{replay.whiteName}</b>
	</a>
	<img class="flag" src="/flags/{replay.whiteCountry}.png" alt="" />
	<span>({replay.whiteElo})</span>
	<span class={whiteClass}>
		{formatElo(replay.whiteEloDiff)}
	</span>
</div>
<div class="side-table-header-elem">
	<a href="/players/{replay.blackId}" class="text-ul">
		<b>{replay.blackName}</b>
	</a>
	<img class="flag" src="/flags/{replay.blackCountry}.png" alt="" />
	<span>({replay.blackElo})</span>
	<span class={blackClass}>
		{formatElo(replay.blackEloDiff)}
	</span>
</div>