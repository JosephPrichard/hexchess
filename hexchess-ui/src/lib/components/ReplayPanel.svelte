<script lang="ts">
	import { UntypedGameModeNameMap, type ReplayModel } from '$lib/api/models';
	import { formatCause, formatEloDiff, formatResult, getReplayColors } from '$lib/utils/format';

	const { replay }: { replay: ReplayModel } = $props();

	const [result, cause, mode] = $derived.by(() => [formatResult(replay.result), formatCause(replay.cause), UntypedGameModeNameMap[replay.mode]]);
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
	<div class="mode">
		{mode}
	</div>
	<div class="result-cause">
		<b class="result">
			{result}
		</b>
		<span class="cause">
		by {cause}
	</span>
	</div>
</div>

<style>
	.mode {
        padding-top: 2px;
        padding-bottom: 2px;
	}

	.result-cause {
		padding-top: 2px;
		padding-bottom: 2px;
	}

    .result {
        font-size: 14px;
    }

    .cause {
        font-size: 14px;
    }
</style>