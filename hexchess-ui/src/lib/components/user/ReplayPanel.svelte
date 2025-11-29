<script lang="ts">
	import { formatElo } from '$lib/api/models';
	import type { ReplayModel } from '$lib/api/models';

	const { replay }: { replay: ReplayModel } = $props();
	
	const [whiteClass, blackClass] = $derived.by(() => {
		switch (replay.result) {
		case 'WHITE_WINS':
			return ['green-color', 'red-color'];
		case 'BLACK_WINS':
			return ['red-color', 'green-color'];
		case 'DRAW':
			return ['yellow-color', 'yellow-color'];
		default:
			console.error('Unknown result case', replay.result);
			return ['', ''];
		}
	});
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