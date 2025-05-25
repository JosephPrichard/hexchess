<script lang="ts">
	import { type ChessBoard, type PieceMove, type ReplayView } from '$lib/models';
	import Banner from '$lib/components/Banner.svelte';
	import ChessBoardView from '$lib/components/chess/Board.svelte';
	import { translateBoard } from '$lib/chess';
	import RightIcon from '$lib/components/icons/RightIcon.svelte';
	import LeftIcon from '$lib/components/icons/LeftIcon.svelte';
	import FlipIcon from '$lib/components/icons/FlipIcon.svelte';
	import { formatCause, formatElo, getResultClasses } from '$lib/format';
	import { formatReplayResult } from '$lib/format.js';
	import { getReplayMoveList, unwrap } from '$lib/api';
	import { createMessage } from '$lib/error';
	import { getNotificationsContext } from '$lib/context';
	import MoveList from '$lib/components/chess/MoveList.svelte';

	export interface ReplayProps {
		replay: ReplayView;
		initialBoard: ChessBoard;
	}

	const { data }: { data: ReplayProps } = $props();
	const { replay, initialBoard } = $derived(data);
	const [whiteClass, blackClass] = $derived(getResultClasses(replay.result));

	const { addNotification } = getNotificationsContext();

	let isWhitePerspective = $state(true);
	let moveIndex: number | undefined = $state(undefined);
	let moveList: PieceMove[] = $state([]);

	let boardCache = new Map<number, ChessBoard>();

	async function initMoveList(replayId: string) {
		const { ok, resp, err } = await unwrap(getReplayMoveList(replayId));
		if (ok && resp) {
			moveList = resp;
		} else {
			const message = 'Failed to load replay move list: ' + createMessage(err);
			addNotification({ type: 'string', message, isSuccess: false, duration: 3000 });
		}
	}

	$effect(() => {
		initMoveList(String(replay.id));
	});

	const board = $derived.by(() => {
		if (!moveIndex) {
			return initialBoard;
		}

		let board = boardCache.get(moveIndex);
		if (board !== undefined) {
			return board;
		}

		board = translateBoard(moveIndex, moveList, initialBoard);

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
		} else if (moveIndex < moveList.length - 1) {
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
			<MoveList moveList={moveList} {onSelectMove} selectedMoveIndex={moveIndex} />
			<div class="side-table-footer">
				<div class="move-table-nav-buttons">
					<button title="Previous Move" class="button-transparent" style:padding-top="5px" onclick={onClickLeft}>
						<LeftIcon />
					</button>
					<button title="Flip Board" class="button-transparent" style:padding-top="5px" onclick={onClickFlip}>
						<FlipIcon />
					</button>
					<button title="Next Move" class="button-transparent" style:padding-top="5px" onclick={onClickRight}>
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