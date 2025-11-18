<script lang="ts">
	import { formatElo, getResultClasses } from '../../services/format.js';
	import type { ReplayModel } from '../../api/model';

	const { replay }: { replay: ReplayModel } = $props();
	const [whiteClass, blackClass] = $derived(getResultClasses(replay.result));
</script>

<div class="side-table-header">
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
</div>