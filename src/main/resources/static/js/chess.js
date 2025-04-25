var piece = {
    empty: 0,
    whitePawn: 1,
    blackPawn: 2,
    whiteKnight: 3,
    blackKnight: 4,
    whiteBishop: 5,
    blackBishop: 6,
    whiteRook: 7,
    blackRook: 8,
    whiteQueen: 9,
    blackQueen: 10,
    whiteKing: 11,
    blackKing: 12
};

var names = {};
names[piece.whitePawn] = "white-pawn";
names[piece.blackPawn] = "black-pawn";
names[piece.whiteKnight] = "white-knight";
names[piece.blackKnight] = "black-knight";
names[piece.whiteBishop] = "white-bishop";
names[piece.blackBishop] = "black-bishop";
names[piece.whiteRook] = "white-rook";
names[piece.blackRook] = "black-rook";
names[piece.whiteQueen] = "white-queen";
names[piece.blackQueen] = "black-queen";
names[piece.whiteKing] = "white-king";
names[piece.blackKing] = "black-king";

var symbols = {};
symbols[piece.whitePawn] = 'p';
symbols[piece.blackPawn] = 'p';
symbols[piece.whiteKnight] = 'n';
symbols[piece.blackKnight] = 'n';
symbols[piece.whiteBishop] = 'b';
symbols[piece.blackBishop] = 'b';
symbols[piece.whiteRook] = 'r';
symbols[piece.blackRook] = 'r';
symbols[piece.whiteQueen] = 'q';
symbols[piece.blackQueen] = 'q';
symbols[piece.whiteKing] = 'k';
symbols[piece.blackKing] = 'k';

function displayChessBoard($board, board, isBlackPerspective) {
    $board.empty();

    var height = 64;
    var width = height * 1.2;
    var verticalFileOffsets = [5, 4, 3, 2, 1, 0, 1, 2, 3, 4, 5]; // file -> vertical offset

    var colors = ["rgb(255, 207, 159)", "rgb(233, 172, 112)", "rgb(210,140,69)"]; // LIGHT, MEDIUM, DARK
    var colorsOffset = [0, 1, 2, 0, 1, 2, 1, 0, 2, 1, 0]; // file -> color

    $board.css({
        width: 11 * height + "px",
        height: 11 * height + "px"
    });

    for (var file = 0; file < board.pieces.length; file++) {
        var piecesFile = board.pieces[file];
        for (var rank = 0; rank < piecesFile.length; rank++) {
            var piece = piecesFile[rank];
            var top = (rank * height) + (verticalFileOffsets[file] * height / 2);
            if (isBlackPerspective) {
                top = (10 * height) - top;
            }
            var left = file * (height - 8);
            var index = (colorsOffset[file] + rank) % 3;
            var bgColor = colors[index];
            var piecename = names[piece];

            var hexagon = $("<div/>")
                .addClass("hexagon")
                .attr("id", "hexagon-" + file + "-" + rank)
                .attr("data-rank", rank)
                .attr("data-file", file)
                .css({
                    top: top + "px",
                    left: left + "px",
                    width: width + "px",
                    height: height + "px",
                    background: bgColor
                })
                .appendTo($board);

            if (piecename) {
                $("<img alt='' src=''>")
                    .attr("id", "piece-" + file + "-" + rank)
                    .attr("src", "/static/images/pieces/" + piecename + ".png")
                    .attr("alt", "")
                    .attr("draggable", false)
                    .addClass("piece-img")
                    .appendTo(hexagon);
            }
        }
    }
}

var files = ['a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'];

function stringOfMove(move) {
    var symbol = symbols[move.piece] || '?';
    var toFile = files[move.to.file];
    var toRank = String(move.to.rank + 1);

    return symbol + toFile + toRank;
}

function displayMoveList($moveList, moveList, handleOnClickMove) {
    $moveList.empty();

    var ply = 1;

    for (var i = 0; i < moveList.length; i += 2) {
        var $moveOne = $("<div/>")
            .addClass("move")
            .attr("id", "move-" + i)
            .html(stringOfMove(moveList[i]));
        $moveOne.on("click", function(i, $moveOne) {
            return function() {
                handleOnClickMove(i, $moveOne);
            };
        }(i, $moveOne));

        var $moveTwo = $("<div/>")
            .addClass("move")
            .attr("id", "move-" + (i + 1));
        if (moveList[i + 1]) {
            $moveTwo.html(stringOfMove(moveList[i + 1]))
                .on("click", function(i, $moveTwo) {
                    return function() {
                        handleOnClickMove(i, $moveTwo);
                    };
                }(i + 1, $moveTwo));
        }

        $("<div/>").addClass("move-row")
            .append($("<div/>").addClass("move-number").text(ply + "."))
            .append($moveOne)
            .append($moveTwo)
            .appendTo($moveList);

        ply++;
    }
}