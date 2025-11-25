package web

import "hexchess-svc/pb"

func MakePbGameOutputError(gameID string, err error) *pb.GameOutput {
	return &pb.GameOutput{
		GameId: gameID,
		Value: &pb.GameOutput_Error{
			Error: &pb.ErrorOutput{
				Message: err.Error(),
			},
		},
	}
}

func MakePbGameOutputInit(gameID string, state *pb.ChessState, self *pb.PlayerState) *pb.GameOutput {
	return &pb.GameOutput{
		GameId: gameID,
		Value: &pb.GameOutput_Init{
			Init: &pb.InitOutput{
				State: state,
				Self:  self,
			},
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

func MakePbGameOutputForfeit(gameID string) *pb.GameOutput {
	return &pb.GameOutput{
		GameId: gameID,
		Value:  &pb.GameOutput_Forfeit{Forfeit: &pb.ForfeitOutput{}},
	}
}

func MakePbGameOutputMove(gameID string, move *pb.HistMove, game *pb.ChessGame) *pb.GameOutput {
	return &pb.GameOutput{
		GameId: gameID,
		Value: &pb.GameOutput_Move{
			Move: &pb.MoveOutput{
				HistMove: move,
				Game:     game,
			},
		},
	}
}

func MakePbGameOutputChat(gameID, message string) *pb.GameOutput {
	return &pb.GameOutput{
		GameId: gameID,
		Value: &pb.GameOutput_Chat{
			Chat: &pb.ChatOutput{
				Message: message,
			},
		},
	}
}
