package model

import (
	"errors"
	"fmt"
	"hexchess-svc/chess"
	"hexchess-svc/pb"
	"hexchess-svc/utils/enum"
	"time"
)

// Piece

func DeserializePieces(pbPieces []uint32) []chess.Piece {
	if len(pbPieces) == 0 {
		return nil
	}
	pieces := make([]chess.Piece, 0, len(pbPieces))
	for _, p := range pbPieces {
		pieces = append(pieces, chess.Piece(p))
	}
	return pieces
}

func SerializePieces(pieces []chess.Piece) []uint32 {
	pbPieces := make([]uint32, 0, len(pieces))
	for _, p := range pieces {
		pbPieces = append(pbPieces, uint32(p))
	}
	return pbPieces
}

// Move

func DeserializeMove(pbMove *pb.Move) chess.Move {
	if pbMove == nil {
		return chess.Move{}
	}
	return chess.Move{
		From: chess.Hex{
			File: uint32(pbMove.FromFile),
			Rank: uint32(pbMove.FromRank),
		},
		To: chess.Hex{
			File: uint32(pbMove.ToFile),
			Rank: uint32(pbMove.ToRank),
		},
		Promotion: chess.Promotion(pbMove.Promotion),
	}
}

// PiecesMoves

func DeserializePiecesMoves(pbMoves []*pb.PieceMoves) []chess.PieceMoves {
	if len(pbMoves) == 0 {
		return nil
	}
	pmsArr := make([]chess.PieceMoves, 0, len(pbMoves))
	for _, pbMove := range pbMoves {
		if pbMove == nil {
			continue
		}
		moves := make([]chess.Hex, 0, len(pbMove.Moves))
		for _, hex := range pbMove.Moves {
			if hex == nil {
				continue
			}
			moves = append(moves, chess.Hex{File: hex.File, Rank: hex.Rank})
		}
		pms := chess.PieceMoves{
			Piece: chess.Piece(pbMove.Piece),
			From:  chess.Hex{File: pbMove.FromFile, Rank: pbMove.FromRank},
			Moves: moves,
		}
		pmsArr = append(pmsArr, pms)
	}
	return pmsArr
}

func SerializePiecesMoves(moves []chess.PieceMoves) []*pb.PieceMoves {
	if len(moves) == 0 {
		return nil
	}
	pbMoves := make([]*pb.PieceMoves, 0, len(moves))
	for _, pm := range moves {
		pbHexes := make([]*pb.Hex, 0, len(pm.Moves))
		for _, h := range pm.Moves {
			pbHexes = append(pbHexes, &pb.Hex{File: h.File, Rank: h.Rank})
		}
		pbMoves = append(pbMoves, &pb.PieceMoves{
			Piece:    uint32(pm.Piece),
			FromFile: pm.From.File,
			FromRank: pm.From.Rank,
			Moves:    pbHexes,
		})
	}
	return pbMoves
}

// Board

func DeserializeBoard(pbBoard *pb.ChessBoard) (chess.Board, error) {
	if pbBoard == nil {
		return chess.Board{}, nil
	}
	board := chess.Board{IsWhiteTurn: pbBoard.IsWhiteTurn}
	for file, bFile := range pbBoard.File {
		for rank, piece := range bFile.Pieces {
			p := chess.Piece(piece)
			if err := board.SetPiece(uint32(file), uint32(rank), p); err != nil {
				return chess.Board{}, err
			}
		}
	}
	return board, nil
}

func SerializeBoard(board *chess.Board) *pb.ChessBoard {
	if board == nil {
		return nil
	}
	files := make([]*pb.BoardFile, 0, chess.Files)
	for file := range chess.Files {
		ranksCount := chess.RanksPerFile[file]
		pieces := make([]uint32, 0, ranksCount)
		for rank := range ranksCount {
			piece, err := board.GetPiece(file, rank)
			if err != nil {
				panic(fmt.Errorf("get piece: %w", err))
			}
			pieces = append(pieces, uint32(piece))
		}
		files = append(files, &pb.BoardFile{Pieces: pieces})
	}
	return &pb.ChessBoard{File: files, IsWhiteTurn: board.IsWhiteTurn}
}

// HistMove

func DeserializeHistMove(pbHm *pb.HistMove) chess.HistMove {
	if pbHm == nil {
		return chess.HistMove{}
	}
	return chess.HistMove{
		PieceMove: chess.PieceMove{
			Piece: chess.Piece(pbHm.Piece),
			From:  chess.Hex{File: uint32(pbHm.FromFile), Rank: uint32(pbHm.FromRank)},
			To:    chess.Hex{File: uint32(pbHm.ToFile), Rank: uint32(pbHm.ToRank)},
		},
		Notation:   pbHm.Notation,
		WhiteTimer: time.Duration(pbHm.WhiteTimerMs) * time.Millisecond,
		BlackTimer: time.Duration(pbHm.BlackTimerMs) * time.Millisecond,
	}
}

func DeserializeHistMoveList(pbMoves []*pb.HistMove) []chess.HistMove {
	if len(pbMoves) == 0 {
		return nil
	}
	moves := make([]chess.HistMove, 0, len(pbMoves))
	for _, pbHm := range pbMoves {
		moves = append(moves, DeserializeHistMove(pbHm))
	}
	return moves
}

func SerializeHistMove(hm chess.HistMove) *pb.HistMove {
	return &pb.HistMove{
		Piece:        uint32(hm.Piece),
		FromFile:     hm.From.File,
		FromRank:     hm.From.Rank,
		ToFile:       hm.To.File,
		ToRank:       hm.To.Rank,
		Notation:     hm.Notation,
		WhiteTimerMs: hm.WhiteTimer.Milliseconds(),
		BlackTimerMs: hm.BlackTimer.Milliseconds(),
	}
}

func SerializeMoveList(moves []chess.HistMove) []*pb.HistMove {
	if len(moves) == 0 {
		return nil
	}
	pbMoveList := make([]*pb.HistMove, 0, len(moves))
	for _, pm := range moves {
		pbMoveList = append(pbMoveList, SerializeHistMove(pm))
	}
	return pbMoveList
}

// Game

func DeserializeGame(pbGame *pb.ChessGame) (g chess.Game, err error) {
	if pbGame == nil {
		return chess.Game{}, err
	}
	board, err := DeserializeBoard(pbGame.Board)
	if err != nil {
		return g, fmt.Errorf("deserialize board %v: %w", pbGame.Board, err)
	}
	return chess.Game{
		TakenWhitePieces: DeserializePieces(pbGame.TakenWhitePieces),
		TakenBlackPieces: DeserializePieces(pbGame.TakenBlackPieces),
		BlackMoves:       DeserializePiecesMoves(pbGame.BlackMoves),
		WhiteMoves:       DeserializePiecesMoves(pbGame.WhiteMoves),
		Moves:            DeserializeHistMoveList(pbGame.Moves),
		Board:            board,
	}, nil
}

func SerializeGame(game *chess.Game) *pb.ChessGame {
	if game == nil {
		return nil
	}
	return &pb.ChessGame{
		TakenWhitePieces: SerializePieces(game.TakenWhitePieces),
		TakenBlackPieces: SerializePieces(game.TakenBlackPieces),
		BlackMoves:       SerializePiecesMoves(game.BlackMoves),
		WhiteMoves:       SerializePiecesMoves(game.WhiteMoves),
		Moves:            SerializeMoveList(game.Moves),
		Board:            SerializeBoard(&game.Board),
	}
}

// MoveHistory

func MarshalMoveHistory(moveHist chess.MoveHistory) ([]byte, error) {
	game := chess.Game{Board: moveHist.InitialBoard}
	game.InitPieceMoves()

	pbInitialGame := SerializeGame(&game)

	var pbMoveSteps []*pb.HistMove
	for _, m := range moveHist.MoveSeq {
		pbMoveSteps = append(pbMoveSteps, SerializeHistMove(m))
	}

	pbMoveHist := &pb.MoveHistory{InitialGame: pbInitialGame, Steps: pbMoveSteps}
	return pbMoveHist.MarshalVT()
}

// Player

func UnmarshalPlayer(bytes []byte) (PlayerState, error) {
	var pbPlayer pb.PlayerState
	if err := pbPlayer.UnmarshalVT(bytes); err != nil {
		return PlayerState{}, err
	}
	return DeserializePlayer(&pbPlayer), nil
}

func MarshalPlayer(player PlayerState) ([]byte, error) {
	return SerializePlayer(player).MarshalVT()
}

func DeserializePlayer(pbPlayer *pb.PlayerState) PlayerState {
	if pbPlayer != nil {
		return PlayerState{ID: pbPlayer.Id, Name: pbPlayer.Name, Country: pbPlayer.Country, Present: true}
	} else {
		return PlayerState{}
	}
}

func SerializePlayer(player PlayerState) *pb.PlayerState {
	var pbPlayer *pb.PlayerState
	if player.Present {
		pbPlayer = &pb.PlayerState{Id: player.ID, Name: player.Name, Country: player.Country}
	}
	return pbPlayer
}

// EndKind

func DeserializeEndKind(pbEndKind pb.EndKind) EndKind {
	switch pbEndKind {
	case pb.EndKind_NOT_ENDED:
		return NotEnded
	case pb.EndKind_FINISHED:
		return Finished
	case pb.EndKind_ABORTED:
		return Aborted
	default:
		panic(fmt.Sprintf("unknown end state: %v", pbEndKind))
	}
}

func SerializeEndKind(endKind EndKind) pb.EndKind {
	switch endKind {
	case NotEnded:
		return pb.EndKind_NOT_ENDED
	case Finished:
		return pb.EndKind_FINISHED
	case Aborted:
		return pb.EndKind_ABORTED
	default:
		panic(fmt.Sprintf("unknown end state: %v", endKind))
	}
}

// Undo

func DeserializeUndoInput(pbInput *pb.UndoInput) (UndoKind, error) {
	var undoKind UndoKind
	switch pbInput.Kind {
	case "CREATE":
		undoKind = UndoCreate
	case "ACCEPT":
		undoKind = UndoAccept
	case "REJECT":
		undoKind = UndoReject
	default:
		return 0, fmt.Errorf("invalid undo kind: %s", pbInput.Kind)
	}
	return undoKind, nil
}

// ChessState

func UnmarshalChessState(bytes []byte) (*ChessState, error) {
	var pbChess pb.ChessState
	if err := pbChess.UnmarshalVT(bytes); err != nil {
		return nil, err
	}
	return DeserializeChessState(&pbChess)
}

func DeserializeChessState(pbChess *pb.ChessState) (*ChessState, error) {
	game, err := DeserializeGame(pbChess.Game)
	if err != nil {
		return nil, err
	}
	initialBoard, err := DeserializeBoard(pbChess.InitialBoard)
	if err != nil {
		return nil, err
	}

	mode, me := enum.Parse(pbChess.Mode, GameModeEnums)
	firstColor, colorErr := enum.Parse(pbChess.FirstColor, GameColorEnums)
	if err := errors.Join(me, colorErr); err != nil {
		return nil, err
	}

	return &ChessState{
		Game:         game,
		InitialBoard: initialBoard,
		UndoState:    UndoState{UndoID: pbChess.UndoId},
		EndState:     DeserializeEndKind(pbChess.EndState),
		ID:           GameID(pbChess.Id),
		WhitePlayer:  DeserializePlayer(pbChess.WhitePlayer),
		BlackPlayer:  DeserializePlayer(pbChess.BlackPlayer),
		FirstColor:   firstColor,
		Mode:         mode,
	}, nil
}

func MarshalChessState(state *ChessState) ([]byte, error) {
	return SerializeChessState(state).MarshalVT()
}

func SerializeChessState(state *ChessState) *pb.ChessState {
	if state == nil {
		return nil
	}
	return &pb.ChessState{
		Id:           state.ID.String(),
		Game:         SerializeGame(&state.Game),
		WhitePlayer:  SerializePlayer(state.WhitePlayer),
		BlackPlayer:  SerializePlayer(state.BlackPlayer),
		FirstColor:   state.FirstColor.String(),
		Mode:         state.Mode.String(),
		InitialBoard: SerializeBoard(&state.InitialBoard),
		UndoId:       state.UndoID,
		EndState:     SerializeEndKind(state.EndState),
	}
}

// ChatMessage

func SerializeChat(chat Chat) *pb.ChatMessage {
	return &pb.ChatMessage{
		Id:      chat.ID,
		Player:  SerializePlayer(chat.Player),
		Message: chat.Message,
		SentAt:  chat.SentAt.Format(time.RFC3339),
	}
}

func MarshalChats(chats []Chat) ([]byte, error) {
	pbChats := make([]*pb.ChatMessage, 0, len(chats))
	for _, chat := range chats {
		pbChats = append(pbChats, SerializeChat(chat))
	}
	return (&pb.ChatMessages{Chats: pbChats}).MarshalVT()
}

func UnmarshalChat(bytes []byte) (Chat, error) {
	var pbChat pb.ChatMessage
	if err := pbChat.UnmarshalVT(bytes); err != nil {
		return Chat{}, err
	}
	madeOn, err := time.Parse(time.RFC3339, pbChat.SentAt)
	if err != nil {
		return Chat{}, err
	}
	return Chat{
		ID:      pbChat.Id,
		Player:  DeserializePlayer(pbChat.Player),
		Message: pbChat.Message,
		SentAt:  madeOn,
	}, nil
}

// chess.Game Input / Output

func MarshalGameOutputError(o ErrorGameOutput) ([]byte, error) {
	pbOutput := &pb.GameOutput{
		GameId:    o.GameID.String(),
		MessageId: o.MessageID,
		Value: &pb.GameOutput_Error{
			Error: &pb.ErrorOutput{Message: o.Error.Error()},
		},
	}
	return pbOutput.MarshalVT()
}

func MarshalGameOutputInit(o InitGameOutput) ([]byte, error) {
	pbOutput := &pb.GameOutput{
		GameId: o.GameID.String(),
		Value: &pb.GameOutput_Init{
			Init: &pb.InitOutput{
				State: SerializeChessState(o.State),
				Self:  SerializePlayer(o.Self),
			},
		},
	}
	return pbOutput.MarshalVT()
}

func SerializeGameOutputPlayers(o PlayersGameOutput) *pb.GameOutput {
	return &pb.GameOutput{
		GameId: o.GameID.String(),
		Value: &pb.GameOutput_Players{
			Players: &pb.PlayersOutput{
				WhitePlayer: SerializePlayer(o.WhitePlayer),
				BlackPlayer: SerializePlayer(o.BlackPlayer),
			},
		},
	}
}

func SerializeGameOutputForfeit(o ForfeitGameOutput) *pb.GameOutput {
	return &pb.GameOutput{
		GameId:    o.GameID.String(),
		MessageId: o.MessageID,
		Value: &pb.GameOutput_Forfeit{
			Forfeit: &pb.ForfeitOutput{
				EndState: SerializeEndKind(o.EndState),
			},
		},
	}
}

func SerializeGameOutputMove(o MoveGameOutput) *pb.GameOutput {
	var game *pb.ChessGame
	if o.State != nil {
		game = SerializeGame(&o.State.Game)
	}
	return &pb.GameOutput{
		GameId:    o.GameID.String(),
		MessageId: o.MessageID,
		Value: &pb.GameOutput_Move{
			Move: &pb.MoveOutput{
				Move:      SerializeHistMove(o.Move),
				Game:      game,
				UpdatedAt: o.UpdatedAt.Format(time.RFC3339),
			},
		},
	}
}

func SerializeGameOutputChat(o ChatGameOutput) *pb.GameOutput {
	return &pb.GameOutput{
		GameId:    o.GameID.String(),
		MessageId: o.MessageID,
		Value: &pb.GameOutput_Chat{Chat: &pb.ChatMessage{
			Player:  SerializePlayer(o.Chat.Player),
			Message: o.Chat.Message,
			SentAt:  o.Chat.SentAt.Format(time.RFC3339),
		}},
	}
}

func SerializeGameOutputUndo(o UndoGameOutput) *pb.GameOutput {
	var game *pb.ChessGame
	if o.State != nil {
		game = SerializeGame(&o.State.Game)
	}
	return &pb.GameOutput{
		GameId:    o.GameID.String(),
		MessageId: o.MessageID,
		Value: &pb.GameOutput_Undo{
			Undo: &pb.UndoOutput{Kind: o.Kind, UndoId: o.UndoID, Game: game},
		},
	}
}

func SerializeReplayOutput(o ReplayGameOutput) *pb.GameOutput {
	return &pb.GameOutput{
		GameId: o.GameID.String(),
		Value: &pb.GameOutput_Replay{Replay: &pb.ReplayOutput{
			Id:           o.Replay.ID,
			WhiteId:      o.Replay.WhiteID,
			BlackId:      o.Replay.BlackID,
			Mode:         o.Replay.Mode.String(),
			Result:       o.Replay.Result.String(),
			Cause:        o.Replay.Cause.String(),
			WinEloDiff:   o.Replay.WinEloDiff,
			LoseEloDiff:  o.Replay.LoseEloDiff,
			PlayedOn:     o.Replay.PlayedOn.Format(time.RFC3339),
			WhiteName:    o.Replay.WhiteName,
			BlackName:    o.Replay.BlackName,
			WhiteCountry: o.Replay.WhiteCountry,
			BlackCountry: o.Replay.BlackCountry,
			WhiteElo:     o.Replay.WhiteElo,
			BlackElo:     o.Replay.BlackElo,
			WhiteEloDiff: o.Replay.WhiteEloDiff,
			BlackEloDiff: o.Replay.BlackEloDiff,
		}},
	}
}
