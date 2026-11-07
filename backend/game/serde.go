package game

import (
	"errors"
	"fmt"
	"hexchess-svc/chess"
	"hexchess-svc/gamedata"
	"hexchess-svc/gameid"
	"hexchess-svc/pb"
	"hexchess-svc/player"
	"hexchess-svc/utils/enum"
	"time"
)

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
	game, err := chess.DeserializeGame(pbChess.Game)
	if err != nil {
		return nil, err
	}
	initialBoard, err := chess.DeserializeBoard(pbChess.InitialBoard)
	if err != nil {
		return nil, err
	}

	mode, me := enum.Parse(pbChess.Mode, gamedata.GameModeEnums)
	firstColor, colorErr := enum.Parse(pbChess.FirstColor, gamedata.GameColorEnums)
	if err := errors.Join(me, colorErr); err != nil {
		return nil, err
	}

	return &ChessState{
		Game:         game,
		InitialBoard: initialBoard,
		UndoID:       pbChess.UndoId,
		EndState:     DeserializeEndKind(pbChess.EndState),
		ID:           gameid.Identifier(pbChess.Id),
		WhitePlayer:  player.DeserializePlayer(pbChess.WhitePlayer),
		BlackPlayer:  player.DeserializePlayer(pbChess.BlackPlayer),
		FirstColor:   firstColor,
		Mode:         mode,
		StartTime:    time.UnixMilli(pbChess.StartTimeMs),
	}, nil
}

func MarshalChessState(state *ChessState) ([]byte, error) {
	return SerializeChessState(state).MarshalVT()
}

func SerializeChessState(state *ChessState) *pb.ChessState {
	if state == nil {
		return nil
	}

	startTimeMs := state.StartTime.UnixMilli()

	return &pb.ChessState{
		Id:           state.ID.String(),
		Game:         chess.SerializeGame(&state.Game),
		WhitePlayer:  player.SerializePlayer(state.WhitePlayer),
		BlackPlayer:  player.SerializePlayer(state.BlackPlayer),
		FirstColor:   state.FirstColor.String(),
		Mode:         state.Mode.String(),
		InitialBoard: chess.SerializeBoard(&state.InitialBoard),
		UndoId:       state.UndoID,
		EndState:     SerializeEndKind(state.EndState),
		StartTimeMs:  startTimeMs,
	}
}

// ChatMessage

func SerializeChat(chat Chat) *pb.ChatMessage {
	return &pb.ChatMessage{
		Id:      chat.ID,
		Player:  player.SerializePlayer(chat.Player),
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
		Player:  player.DeserializePlayer(pbChat.Player),
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
				Self:  player.SerializePlayer(o.Self),
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
				WhitePlayer: player.SerializePlayer(o.WhitePlayer),
				BlackPlayer: player.SerializePlayer(o.BlackPlayer),
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
		game = chess.SerializeGame(&o.State.Game)
	}
	return &pb.GameOutput{
		GameId:    o.GameID.String(),
		MessageId: o.MessageID,
		Value: &pb.GameOutput_Move{
			Move: &pb.MoveOutput{
				Move:      chess.SerializeHistMove(o.Move),
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
			Player:  player.SerializePlayer(o.Chat.Player),
			Message: o.Chat.Message,
			SentAt:  o.Chat.SentAt.Format(time.RFC3339),
		}},
	}
}

func SerializeGameOutputUndo(o UndoGameOutput) *pb.GameOutput {
	var game *pb.ChessGame
	if o.State != nil {
		game = chess.SerializeGame(&o.State.Game)
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
