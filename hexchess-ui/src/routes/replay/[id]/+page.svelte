<script lang="ts">
	import Banner from '$lib/components/Banner.svelte';
	import Board from '$lib/components/chess/Board.svelte';
	import RightIcon from '$lib/components/icons/RightIcon.svelte';
	import LeftIcon from '$lib/components/icons/LeftIcon.svelte';
	import FlipIcon from '$lib/components/icons/FlipIcon.svelte';
	import { createMessage } from '$lib/utils/error';
	import { getNotificationsContext } from '$lib/utils/context';
	import MoveList from '$lib/components/chess/MoveList.svelte';
	import ReplayPanel from '$lib/components/user/ReplayPanel.svelte';
	import { type ChessBoard, type MoveStep, type PieceMove } from '$lib/api/messages';
	import type { ReplayModel } from '$lib/api/model';
	import services from '$lib/api/services';
	import { createMoveState } from '$lib/state/move.svelte';

	export interface ReplayProps {
		replay: ReplayModel;
	}

	const { data }: { data: ReplayProps } = $props();
	const { replay } = $derived(data);

	const { addNotification } = getNotificationsContext();

	let moveState = createMoveState();

	let moveSteps: MoveStep[] = $state([]);
	let initialBoard: ChessBoard | undefined = $state(undefined);

	async function initMoveList(replayId: string) {
		const [data, err] = await services.getReplayMoveHistory(replayId);
		if (data) {
			moveSteps = data.moveSteps;
			initialBoard = data.initialBoard;
		} else {
			const message = 'Failed to load replay move list: ' + createMessage(err);
			addNotification({ type: 'string', message, isSuccess: false, duration: 3000 });
		}
	}

	$effect(() => {
		initMoveList(String(replay.id));
	});
	$effect(() => {
		moveState.updateMoveCount(moveSteps.length);
	});

	const board = $derived.by(() => moveState.value.moveIndex !== undefined ? moveSteps[moveState.value.moveIndex]?.game?.board : initialBoard);
	const moveList = $derived.by(() => {
		const moveList: PieceMove[] = [];
		for (const step of moveSteps) {
			if (step?.move) {
				moveList.push(step.move);
			}
		}
		return moveList;
	});
</script>

<svelte:head>
	<title>Replay - Hexchess</title>
</svelte:head>
<Banner />
<div class="center-horizontal-container">
	<div class="center-vertical-container" style="align-items: stretch;">
		<Board {board} draggable="none" isWhitePerspective={moveState.value.isWhitePerspective} />
		<div class="side-table">
			<ReplayPanel replay={replay} />
			<MoveList moveList={moveList} onSelectMove={moveState.selectMove} selectedMoveIndex={moveState.value.moveIndex} />
			<div class="side-table-footer">
				<div class="move-table-nav-buttons">
					<button title="Previous Move" class="button-transparent" style:padding-top="5px" onclick={moveState.goLeft}>
						<LeftIcon />
					</button>
					<button title="Flip Board" class="button-transparent" style:padding-top="5px" onclick={moveState.flip}>
						<FlipIcon />
					</button>
					<button title="Next Move" class="button-transparent" style:padding-top="5px" onclick={moveState.goRight}>
						<RightIcon />
					</button>
				</div>
			</div>
		</div>
	</div>
</div>