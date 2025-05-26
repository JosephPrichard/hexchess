<script lang="ts">
	import { type ChessBoard, type PieceMove, type ReplayView } from '$lib/models';
	import Banner from '$lib/components/Banner.svelte';
	import Board from '$lib/components/chess/Board.svelte';
	import { translateBoard } from '$lib/chess';
	import RightIcon from '$lib/components/icons/RightIcon.svelte';
	import LeftIcon from '$lib/components/icons/LeftIcon.svelte';
	import FlipIcon from '$lib/components/icons/FlipIcon.svelte';
	import { getReplayMoveList, unwrap } from '$lib/api';
	import { createMessage } from '$lib/error';
	import { getNotificationsContext } from '$lib/context';
	import MoveList from '$lib/components/chess/MoveList.svelte';
	import ReplayPanel from '$lib/components/user/ReplayPanel.svelte';

	export interface ReplayProps {
		replay: ReplayView;
		initialBoard: ChessBoard;
	}

	const { data }: { data: ReplayProps } = $props();
	const { replay, initialBoard } = $derived(data);

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
		<Board {board} {isWhitePerspective} />
		<div class="side-table">
			<ReplayPanel replay={replay} />
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