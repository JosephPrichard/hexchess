function initReplayPageGivenBoard(initialBoard) {
    var boardCache = [];
    var index = undefined;
    var isFlipped = false;
    var $selectedMove = undefined;

    var moveList = JSON.parse($("#move-list-data").html());

    var $board = $("#chess-board");
    var $moveList = $("#replay-move-list");

    function getBoard(index) {
        if (boardCache[index] !== undefined) {
            return boardCache[index];
        }
        var board = JSON.parse(JSON.stringify(initialBoard));
        for (var i = 0; i <= index; i++) {
            var move = moveList[i];
            var toRank = move.to.rank;
            var toFile = move.to.file;
            var fromRank = move.from.rank;
            var fromFile = move.from.file;

            board.pieces[toFile][toRank] = board.pieces[fromFile][fromRank];
            board.pieces[fromFile][fromRank] = piece.empty;
        }
        boardCache[index] = board;
        return board;
    }

    function handleOnMove(newIndex, $move) {
        var board = getBoard(newIndex);
        index = newIndex;
        displayChessBoard($board, board, isFlipped);

        if ($selectedMove !== undefined) {
            $($selectedMove).removeClass("selected-move");
        }
        $selectedMove = $move;
        $move.addClass("selected-move");
    }

    function onClickFlip() {
        isFlipped = !isFlipped;
        var board = index !== undefined ? getBoard(index) : initialBoard;
        displayChessBoard($board, board, isFlipped);
    }

    function onClickLeft() {
        if (index === undefined) {
            index = 0;
        } else if (index > 0) {
            index--;
        } else {
            return;
        }
        handleOnMove(index, $("#move-" + index));
    }

    function onClickRight() {
        if (index === undefined) {
            index = 0;
        } else if (index < moveList.length - 1) {
            index++;
        } else {
            return;
        }
        handleOnMove(index, $("#move-" + index));
    }

    $("#flip-board-button").on("click", onClickFlip);
    $("#left-button").on("click", onClickLeft);
    $("#right-button").on("click", onClickRight);

    displayChessBoard($board, initialBoard, isFlipped);
    displayMoveList($moveList, moveList, handleOnMove);
}

function initReplayPage() {
    $.getJSON("/static/initial-board", function(initialBoard) {
        initReplayPageGivenBoard(initialBoard);
    });
}
