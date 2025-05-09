<script lang="ts">
    import { type ChessBoard, type ReplayView } from '$lib/models';
    import Banner from '$lib/components/Banner.svelte';
    import MoveListView from '$lib/components/chess/MoveListView.svelte';
    import ChessBoardView from '$lib/components/chess/ChessBoardView.svelte';
    import { translateBoard } from '$lib/chess';
    import RightIcon from '$lib/components/icons/RightIcon.svelte';
    import LeftIcon from '$lib/components/icons/LeftIcon.svelte';
    import FlipIcon from '$lib/components/icons/FlipIcon.svelte';

    export interface ReplayProps {
        replay: ReplayView;
        initialBoard: ChessBoard;
    }

    const { data }: { data: ReplayProps } = $props();
    const { replay, initialBoard } = data;

    let isBlackPerspective = $state(false);
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
        isBlackPerspective = !isBlackPerspective;
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
        <ChessBoardView board={board} isBlackPerspective={isBlackPerspective} />
        <div class="move-table">
            <div class="move-table-header">
                <div class="move-table-header-elem">
                    <a href={`/players/${replay.whiteId}`} class="text-ul">
                        <b>{replay.whiteName}</b>
                    </a>
                    <img class="flag" src={`/static/images/flags/${replay.whiteCountry}.png`} alt="" />
                    <span>({replay.whiteElo})</span>
                    <span class={replay.whiteEloColor}>
                    {replay.whiteEloDiff}
                </span>
                </div>
                <div class="move-table-header-elem">
                    <a href={`/players/${replay.blackId}`} class="text-ul">
                        <b>{replay.blackName}</b>
                    </a>
                    <img class="flag" src={`/static/images/flags/${replay.blackCountry}.png`} alt="" />
                    <span>({replay.blackElo})</span>
                    <span class={replay.blackEloColor}>
                    {replay.blackEloDiff}
                </span>
                </div>
                <div class="move-table-header-elem">
                    {replay.result} &#8226; {replay.cause}
                </div>
                <div class="move-table-header-elem">
                    {replay.playedOn}
                </div>
            </div>
            <MoveListView moveList={replay.moveList || []} onSelectMove={onSelectMove} selectedMoveIndex={moveIndex} />
            <div id="move-table-buttons" class="move-table-buttons">
                <div class="move-table-nav-buttons">
                    <button id="left-button" class="button-transparent" onclick={onClickLeft}>
                        <LeftIcon />
                    </button>
                    <button id="flip-board-button" class="button-transparent" onclick={onClickFlip}>
                        <FlipIcon />
                    </button>
                    <button id="right-button" class="button-transparent" onclick={onClickRight}>
                        <RightIcon />
                    </button>
                </div>
            </div>
        </div>
    </div>
</div>