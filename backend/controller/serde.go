package controller

import (
	"fmt"
	"hexchess-svc/chess"
	"hexchess-svc/model"
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

func SerializeGameOutputError(gameID model.GameID, messageID string, err error) *pb.GameOutput {
	return &pb.GameOutput{
		GameId:    gameID.String(),
		MessageId: messageID,
		Value: &pb.GameOutput_Error{
			Error: &pb.ErrorOutput{Message: err.Error()},
		},
	}
}

func SerializeGameOutputInit(gameID model.GameID, state *pb.ChessState, self *pb.PlayerState) *pb.GameOutput {
	return &pb.GameOutput{
		GameId: gameID.String(),
		Value: &pb.GameOutput_Init{
			Init: &pb.InitOutput{State: state, Self: self},
		},
	}
}

func SerializeGameOutputPlayers(gameID model.GameID, white, black *pb.PlayerState) *pb.GameOutput {
	return &pb.GameOutput{
		GameId: gameID.String(),
		Value: &pb.GameOutput_Players{
			Players: &pb.PlayersOutput{
				WhitePlayer: white,
				BlackPlayer: black,
			},
		},
	}
}

func SerializeGameOutputForfeit(gameID model.GameID, messageID string, endState model.EndKind) *pb.GameOutput {
	return &pb.GameOutput{
		GameId:    gameID.String(),
		MessageId: messageID,
		Value: &pb.GameOutput_Forfeit{
			Forfeit: &pb.ForfeitOutput{
				EndState: model.SerializeEndKind(endState),
			},
		},
	}
}

func SerializeGameOutputMove(gameID model.GameID, messageID string, move *pb.HistMove, game *pb.ChessGame, updatedAt time.Time) *pb.GameOutput {
	return &pb.GameOutput{
		GameId:    gameID.String(),
		MessageId: messageID,
		Value: &pb.GameOutput_Move{
			Move: &pb.MoveOutput{
				Move:      move,
				Game:      game,
				UpdatedAt: updatedAt.Format(time.RFC3339),
			},
		},
	}
}

func SerializeGameOutputChat(gameID model.GameID, messageID string, chat model.Chat) *pb.GameOutput {
	return &pb.GameOutput{
		GameId:    gameID.String(),
		MessageId: messageID,
		Value: &pb.GameOutput_Chat{Chat: &pb.ChatMessage{
			Player:  model.SerializePlayer(chat.Player),
			Message: chat.Message,
			SentAt:  chat.SentAt.Format(time.RFC3339),
		}},
	}
}

func SerializeGameOutputUndo(gameID model.GameID, messageID string, undoKind string, undoID int64, state *model.ChessState) *pb.GameOutput {
	var game *pb.ChessGame
	if state != nil {
		game = chess.SerializeGame(&state.Game)
	}
	return &pb.GameOutput{
		GameId:    gameID.String(),
		MessageId: messageID,
		Value: &pb.GameOutput_Undo{
			Undo: &pb.UndoOutput{
				Kind: undoKind, UndoId: undoID, Game: game,
			},
		},
	}
}
