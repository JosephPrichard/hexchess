<script lang="ts">
	import { type ChessBoard, type ReplayView } from '$lib/models';
	import Banner from '$lib/components/Banner.svelte';
	import MoveListView from '$lib/components/chess/MoveList.svelte';
	import ChessBoardView from '$lib/components/chess/Board.svelte';
	import { translateBoard } from '$lib/chess';
	import RightIcon from '$lib/components/icons/RightIcon.svelte';
	import LeftIcon from '$lib/components/icons/LeftIcon.svelte';
	import FlipIcon from '$lib/components/icons/FlipIcon.svelte';
	import { formatCause, formatElo, getResultClasses } from '$lib/format';
	import { formatReplayResult } from '$lib/format.js';

	export interface ReplayProps {
		replay: ReplayView;
		initialBoard: ChessBoard;
	}

	const { data }: { data: ReplayProps } = $props();
	const { replay, initialBoard } = $derived(data);
	const [whiteClass, blackClass] = $derived(getResultClasses(replay.result));

	let isWhitePerspective = $state(true);
	let moveIndex: number | undefined = $state(undefined);

	let boardCache = new Map<number, ChessBoard>();

	const board = $derived.by(() => {
		if (!moveIndex) {
			return initialBoard;
		}

		let board = boardCache.get(moveIndex);
		if (board !== undefined) {
			return board;
		}

		board = translateBoard(moveIndex, replay.moveList!, initialBoard);

		boardCache.set(moveIndex, board);
		return board;
	});

	function onSelectMove(i: number) {
		moveIndex = i;
	}

	function onClickFlip() {
		isWhitePerspective = !isWhitePerspective;
	}

	function onClickLeft() {
		if (moveIndex === undefined) {
			moveIndex = 0;
		} else if (moveIndex > 0) {
			moveIndex--;
		}
	}

	function onClickRight() {
		if (moveIndex === undefined) {
			moveIndex = 0;
		} else if (moveIndex < replay.moveList!.length - 1) {
			moveIndex++;
		}
	}
</script>

<svelte:head>
	<title>Replay - Hexchess</title>
</svelte:head>
<Banner />
<div class="center-horizontal-container">
	<div class="center-vertical-container" style="align-items: stretch;">
		<ChessBoardView {board} {isWhitePerspective} />
		<div class="side-table">
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
				<div class="side-table-header-elem">
					{formatReplayResult(replay.result)} &#8226; {formatCause(replay.cause)}
				</div>
				<div class="side-table-header-elem">
					{replay.playedOn}
				</div>
			</div>
			<div class="growing-scrollbox">
				<MoveListView moveList={replay.moveList || []} {onSelectMove} selectedMoveIndex={moveIndex} />
			</div>
			<div class="side-table-footer">
				<div class="move-table-nav-buttons">
					<button class="button-transparent" style:padding-top="5px" onclick={onClickLeft}>
						<LeftIcon />
					</button>
					<button class="button-transparent" style:padding-top="5px" onclick={onClickFlip}>
						<FlipIcon />
					</button>
					<button class="button-transparent" style:padding-top="5px" onclick={onClickRight}>
						<RightIcon />
					</button>
				</div>
			</div>
		</div>
	</div>
</div>

<style>
	.move-table-nav-buttons {
		display: flex;
		flex-direction: row;
		gap: 10px;
		align-items: center;
		justify-content: center;
	}
</style>