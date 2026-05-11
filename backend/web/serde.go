package web

import (
	"fmt"
	"time"

	"hexchess-svc/pb"
	svc "hexchess-svc/service"
)

func DeserializeUndoInput(pbInput *pb.UndoInput) (svc.UndoKind, error) {
	var undoKind svc.UndoKind
	switch pbInput.Kind {
	case "CREATE":
		undoKind = svc.UndoCreate
	case "ACCEPT":
		undoKind = svc.UndoAccept
	case "REJECT":
		undoKind = svc.UndoReject
	default:
		return 0, fmt.Errorf("invalid undo kind: %s", pbInput.Kind)
	}
	return undoKind, nil
}

func SerializeGameOutputError(gameID string, err error) *pb.GameOutput {
	return &pb.GameOutput{
		GameId: gameID,
		Value: &pb.GameOutput_Error{
			Error: &pb.ErrorOutput{Message: err.Error()},
		},
	}
}

func SerializeGameOutputInit(gameID string, state *pb.ChessState, self *pb.PlayerState) *pb.GameOutput {
	return &pb.GameOutput{
		GameId: gameID,
		Value: &pb.GameOutput_Init{
			Init: &pb.InitOutput{State: state, Self: self},
		},
	}
}

func SerializeGameOutputPlayers(gameID string, white, black *pb.PlayerState) *pb.GameOutput {
	return &pb.GameOutput{
		GameId: gameID,
		Value: &pb.GameOutput_Players{
			Players: &pb.PlayersOutput{
				WhitePlayer: white,
				BlackPlayer: black,
			},
		},
	}
}

func SerializeGameOutputForfeit(gameID string, endState svc.EndKind) *pb.GameOutput {
	return &pb.GameOutput{
		GameId: gameID,
		Value: &pb.GameOutput_Forfeit{
			Forfeit: &pb.ForfeitOutput{
				EndState: svc.SerializeEndKind(endState),
			},
		},
	}
}

func SerializeGameOutputMove(gameID string, move *pb.HistMove, game *pb.ChessGame, updatedAt time.Time) *pb.GameOutput {
	return &pb.GameOutput{
		GameId: gameID,
		Value: &pb.GameOutput_Move{
			Move: &pb.MoveOutput{
				Move:      move,
				Game:      game,
				UpdatedAt: updatedAt.Format(time.RFC3339),
			},
		},
	}
}

func SerializeGameOutputChat(gameID string, chat svc.Chat) *pb.GameOutput {
	return &pb.GameOutput{
		GameId: gameID,
		Value: &pb.GameOutput_Chat{Chat: &pb.ChatMessage{
			Player:  svc.SerializePlayer(chat.Player),
			Message: chat.Message,
			SentAt:  chat.SentAt.Format(time.RFC3339),
		}},
	}
}

func SerializeGameOutputUndo(gameID string, undoKind string, undoID int64, state *svc.ChessState) *pb.GameOutput {
	var game *pb.ChessGame
	if state != nil {
		game = chess.SerializeGame(&state.Game)
	}
	return &pb.GameOutput{
		GameId: gameID,
		Value: &pb.GameOutput_Undo{
			Undo: &pb.UndoOutput{
				Kind: undoKind, UndoId: undoID, Game: game,
			},
		},
	}
}
