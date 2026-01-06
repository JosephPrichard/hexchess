<script lang="ts">
	import type { ReplayModel } from '$lib/api/models';
	import { formatEloDiff, getReplayColors } from '$lib/utils/format';

	const { replay }: { replay: ReplayModel } = $props();
	
	const [whiteClass, blackClass] = $derived.by(() => getReplayColors(replay.result));
</script>

<div class="side-table-header">
	<div class="side-table-header-elem">
		<a href="/players/{replay.whiteId}" class="text-ul">
			<b>{replay.whiteName}</b>
		</a>
		<img class="flag" src="/flags/{replay.whiteCountry}.png" alt="" />
		<span>({Math.round(replay.whiteElo)})</span>
		<span class={whiteClass}>
			{formatEloDiff(replay.whiteEloDiff)}
		</span>
	</div>
	<div class="side-table-header-elem">
		<a href="/players/{replay.blackId}" class="text-ul">
			<b>{replay.blackName}</b>
		</a>
		<img class="flag" src="/flags/{replay.blackCountry}.png" alt="" />
		<span>({Math.round(replay.blackElo)})</span>
		<span class={blackClass}>
			{formatEloDiff(replay.blackEloDiff)}
		</span>
	</div>
</div>