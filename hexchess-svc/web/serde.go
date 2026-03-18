package web

import (
	"time"

	"hexchess-svc/chess"
	"hexchess-svc/pb"
	svc "hexchess-svc/services"
)

func MakePbGameOutputError(gameID string, err error) *pb.GameOutput {
	return &pb.GameOutput{
		GameId: gameID,
		Value: &pb.GameOutput_Error{
			Error: &pb.ErrorOutput{Message: err.Error()},
		},
	}
}

func MakePbGameOutputInit(gameID string, cs *pb.ChessState, self *pb.PlayerState) *pb.GameOutput {
	return &pb.GameOutput{
		GameId: gameID,
		Value: &pb.GameOutput_Init{
			Init: &pb.InitOutput{State: cs, Self: self},
		},
	}
}

func MakePbGameOutputPlayers(gameID string, white, black *pb.PlayerState) *pb.GameOutput {
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

func MakePbGameOutputMove(gameID string, move *pb.HistMove, game *pb.ChessGame, updatedAt time.Time) *pb.GameOutput {
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

func MakePbGameOutputChat(gameID, message string, self svc.PlayerState, sentAt time.Time) (*pb.GameOutput, svc.Chat) {
	o := &pb.GameOutput{
		GameId: gameID,
		Value: &pb.GameOutput_Chat{Chat: &pb.ChatMessage{
			Player:  svc.SerializePlayer(self),
			Message: message,
			SentAt:  sentAt.Format(time.RFC3339),
		}},
	}
	c := svc.Chat{Player: self, Message: message, SentAt: sentAt}
	return o, c
}

func MakePbGameOutputUndo(gameID string, undoKind string, undoID int64, cs *svc.ChessState) *pb.GameOutput {
	var game *pb.ChessGame
	if cs != nil {
		game = chess.SerializeGame(&cs.Game)
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
