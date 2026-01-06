package web

import (
	"hexchess-svc/chess"
	"hexchess-svc/pb"
	"hexchess-svc/services"
	"time"
)

func MakePbGameOutputError(gameID string, err error) *pb.GameOutput {
	return &pb.GameOutput{
		GameId: gameID,
		Value: &pb.GameOutput_Error{
			Error: &pb.ErrorOutput{Message: err.Error()},
		},
	}
}

func MakePbGameOutputInit(gameID string, state *pb.ChessState, self *pb.PlayerState) *pb.GameOutput {
	return &pb.GameOutput{
		GameId: gameID,
		Value: &pb.GameOutput_Init{
			Init: &pb.InitOutput{State: state, Self: self},
		},
	}
}

func MakePbGameOutputBgInit(gameID string, chats []*pb.ChatMsg) *pb.GameOutput {
	return &pb.GameOutput{
		GameId: gameID,
		Value: &pb.GameOutput_BgInit{
			BgInit: &pb.BgInitOutput{Chats: chats},
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

func MakePbGameOutputForfeit(gameID string, replayID int64, finishState *pb.FinishState) *pb.GameOutput {
	return &pb.GameOutput{
		GameId: gameID,
		Value: &pb.GameOutput_Forfeit{Forfeit: &pb.ForfeitOutput{
			ReplayId:    replayID,
			FinishState: finishState,
		}},
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

func MakePbGameOutputChat(gameID, message string, self svc.PlayerState, sentAt time.Time) (*pb.GameOutput, svc.StateChat) {
	o := &pb.GameOutput{
		GameId: gameID,
		Value: &pb.GameOutput_Chat{Chat: &pb.ChatOutput{
			Player:  svc.SerializePlayer(self),
			Message: message,
			SentAt:  sentAt.Format(time.RFC3339),
		}},
	}
	c := svc.StateChat{Player: self, Message: message, SentAt: sentAt}
	return o, c
}

func MakePbGameOutputUndo(gameID string, undoKind string, undoID int64, state *svc.ChessState) *pb.GameOutput {
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
