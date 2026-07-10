package model

import (
	"errors"
	"fmt"
	"hexchess-lib/enum"
	"hexchess-svc/chess"
	"hexchess-svc/pb"
	"time"

	"github.com/google/uuid"
)

// PlayerState

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
	var player PlayerState
	if pbPlayer != nil {
		player = PlayerState{ID: pbPlayer.Id, Name: pbPlayer.Name, Country: pbPlayer.Country, Present: true}
	}
	return player
}

func SerializePlayer(player PlayerState) *pb.PlayerState {
	if player.Present {
		return &pb.PlayerState{Id: player.ID, Name: player.Name, Country: player.Country, IsGuest: IsGuestID(player.ID)}
	}
	return nil
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
	pbChess := pb.ChessStateFromVTPool()
	if err := pbChess.UnmarshalVT(bytes); err != nil {
		return nil, err
	}
	defer pbChess.ReturnToVTPool()

	game, err := chess.DeserializeGame(pbChess.Game)
	if err != nil {
		return nil, err
	}
	initialBoard, err := chess.DeserializeBoard(pbChess.InitialBoard)
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
		Game:         chess.SerializeGame(&state.Game),
		WhitePlayer:  SerializePlayer(state.WhitePlayer),
		BlackPlayer:  SerializePlayer(state.BlackPlayer),
		FirstColor:   state.FirstColor.String(),
		Mode:         state.Mode.String(),
		InitialBoard: chess.SerializeBoard(&state.InitialBoard),
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

// FinishGameEvent

func UnmarshalFinishedGame(bytes []byte) (FinishedGame, error) {
	var pbGameEvent pb.FinishGameEvent
	if err := pbGameEvent.UnmarshalVT(bytes); err != nil {
		return FinishedGame{}, err
	}

	mode, me := enum.Parse(pbGameEvent.GameMode, GameModeEnums)
	replayResult, re := enum.Parse(pbGameEvent.ReplayResult, ReplayResultEnums)
	replayCause, ce := enum.Parse(pbGameEvent.ReplayCause, ReplayCauseEnums)
	if err := errors.Join(me, re, ce); err != nil {
		return FinishedGame{}, err
	}

	board, err := chess.DeserializeBoard(pbGameEvent.Board)
	if err != nil {
		return FinishedGame{}, err
	}

	return FinishedGame{
		GameID:       GameID(pbGameEvent.GameId),
		Board:        board,
		Moves:        chess.DeserializeHistMoveList(pbGameEvent.Moves),
		WhitePlayer:  pbGameEvent.WhitePlayer,
		BlackPlayer:  pbGameEvent.BlackPlayer,
		ReplayMode:   mode,
		ReplayResult: replayResult,
		ReplayCause:  replayCause,
	}, nil
}

func MarshalFinishedGame(event FinishedGame) ([]byte, error) {
	pbEvent := &pb.FinishGameEvent{
		GameId:       event.GameID.String(),
		Board:        chess.SerializeBoard(&event.Board),
		Moves:        chess.SerializeMoveList(event.Moves),
		WhitePlayer:  event.WhitePlayer,
		BlackPlayer:  event.BlackPlayer,
		GameMode:     event.ReplayMode.String(),
		ReplayResult: event.ReplayResult.String(),
		ReplayCause:  event.ReplayCause.String(),
	}
	return pbEvent.MarshalVT()
}

// GameUpdtEvent

func UnmarshalGameMetadataUpdt(bytes []byte) (GameMetadataUpdt, error) {
	var pbGameEvent pb.UpdtMetadataEvent
	if err := pbGameEvent.UnmarshalVT(bytes); err != nil {
		return GameMetadataUpdt{}, err
	}

	mode, me := enum.Parse(pbGameEvent.Mode, GameModeEnums)
	firstColor, colorErr := enum.Parse(pbGameEvent.FirstColor, GameColorEnums)
	if err := errors.Join(me, colorErr); err != nil {
		return GameMetadataUpdt{}, err
	}

	return GameMetadataUpdt{
		GameID:      GameID(pbGameEvent.GameId),
		WhitePlayer: pbGameEvent.WhitePlayer,
		BlackPlayer: pbGameEvent.BlackPlayer,
		FirstColor:  firstColor,
		Mode:        mode,
	}, nil
}

func MarshalGameMetadataUpdt(event GameMetadataUpdt) ([]byte, error) {
	pbEvent := &pb.UpdtMetadataEvent{
		GameId:      event.GameID.String(),
		WhitePlayer: event.WhitePlayer,
		BlackPlayer: event.BlackPlayer,
		FirstColor:  event.FirstColor.String(),
		Mode:        event.Mode.String(),
	}
	return pbEvent.MarshalVT()
}

// MatchCreation

func UnmarshalMatchCreation(bytes []byte) ([]MatchCreation, error) {
	pbMatches := pb.MatchCreationsFromVTPool()
	if err := pbMatches.UnmarshalVT(bytes); err != nil {
		return nil, err
	}
	defer pbMatches.ReturnToVTPool()

	matches := make([]MatchCreation, 0, len(pbMatches.Creations))

	for _, pbMatch := range pbMatches.Creations {
		mode, err := enum.Parse(pbMatch.Mode, GameModeEnums)
		if err != nil {
			return nil, err
		}
		matches = append(matches, MatchCreation{
			GameID:   GameID(pbMatch.GameId),
			GameMode: mode,
			WhiteID:  pbMatch.WhiteId,
			BlackID:  pbMatch.BlackId,
		})
	}

	return matches, nil
}

func MarshalMatchCreations(matches []MatchCreation) ([]byte, error) {
	pbMatches := make([]*pb.MatchCreation, 0, len(matches))

	for _, m := range matches {
		pbMatches = append(pbMatches, &pb.MatchCreation{
			GameId:  m.GameID.String(),
			Mode:    m.GameMode.String(),
			WhiteId: m.WhiteID,
			BlackId: m.BlackID,
		})
	}

	return (&pb.MatchCreations{Creations: pbMatches}).MarshalVT()
}

// AdvanceTournamentEvent

func UnmarshalAdvanceTournamentEvent(bytes []byte) (e AdvanceTournamentEvent, err error) {
	var pbEvent pb.AdvanceTournamentEvent
	if err := pbEvent.UnmarshalVT(bytes); err != nil {
		return e, err
	}
	tournamentKey, err := uuid.Parse(pbEvent.TournamentKey)
	if err != nil {
		return e, err
	}
	eventID, err := uuid.Parse(pbEvent.EventId)
	if err != nil {
		return e, err
	}
	return AdvanceTournamentEvent{TournamentKey: tournamentKey, EventID: eventID}, nil
}

// Replay

func SerializeReplayOutput(gameID string, replay FullReplay) *pb.GameOutput {
	return &pb.GameOutput{
		GameId: gameID,
		Value: &pb.GameOutput_Replay{Replay: &pb.Replay{
			Id:           replay.ID,
			WhiteId:      replay.WhiteID,
			BlackId:      replay.BlackID,
			Mode:         replay.Mode.String(),
			Result:       replay.Result.String(),
			Cause:        replay.Cause.String(),
			WinEloDiff:   replay.WinEloDiff,
			LoseEloDiff:  replay.LoseEloDiff,
			PlayedOn:     replay.PlayedOn.Format(time.RFC3339),
			WhiteName:    replay.WhiteName,
			BlackName:    replay.BlackName,
			WhiteCountry: replay.WhiteCountry,
			BlackCountry: replay.BlackCountry,
			WhiteElo:     replay.WhiteElo,
			BlackElo:     replay.BlackElo,
			WhiteEloDiff: replay.WhiteEloDiff,
			BlackEloDiff: replay.BlackEloDiff,
		}},
	}
}

// TournamentOutput

func SerializeTournamentError(tournamentKey uuid.UUID, err error) *pb.TournamentOutput {
	return &pb.TournamentOutput{
		TournamentKey: tournamentKey.String(),
		Value: &pb.TournamentOutput_Error{
			Error: &pb.ErrorOutput{Message: err.Error()},
		},
	}
}

func SerializeParticipantOutput(tournamentKey uuid.UUID, lbdUser LbdUser) *pb.TournamentOutput {
	return &pb.TournamentOutput{
		TournamentKey: tournamentKey.String(),
		Value: &pb.TournamentOutput_Participant{
			Participant: SerializeLbdUser(lbdUser),
		},
	}
}

func SerializeBeginTournamentCountdown(tournamentKey uuid.UUID) *pb.TournamentOutput {
	return &pb.TournamentOutput{
		TournamentKey: tournamentKey.String(),
		Value: &pb.TournamentOutput_Countdown{
			Countdown: &pb.BeginCountdownOutput{},
		},
	}
}

func SerializeStartTournament(tournamentKey uuid.UUID) *pb.TournamentOutput {
	return &pb.TournamentOutput{
		TournamentKey: tournamentKey.String(),
		Value: &pb.TournamentOutput_Start{
			Start: &pb.StartTourneyOutput{},
		},
	}
}

// LbdUser

func SerializeLbdUser(user LbdUser) *pb.LbdUser {
	return &pb.LbdUser{
		Id:         user.ID,
		Username:   user.Username,
		Country:    user.Country,
		Bio:        user.Bio,
		JoinedOn:   user.JoinedOn.Format(time.RFC3339),
		Elo:        user.Elo,
		HighestElo: user.HighestElo,
		Wins:       user.Wins,
		Losses:     user.Losses,
		Draws:      user.Draws,
		Winrate:    user.Winrate,
		Rank:       user.Rank,
	}
}

func DeserializeLbdUser(user *pb.LbdUser) (LbdUser, error) {
	joinedOn, err := time.Parse(time.RFC3339, user.JoinedOn)
	if err != nil {
		return LbdUser{}, err
	}

	return LbdUser{
		User: User{
			ID: user.Id, Username: user.Username, Country: user.Country, Bio: user.Bio, JoinedOn: joinedOn,
		},
		Elo:        user.Elo,
		HighestElo: user.HighestElo,
		Wins:       user.Wins,
		Losses:     user.Losses,
		Draws:      user.Draws,
		Winrate:    user.Winrate,
		Rank:       user.Rank,
	}, nil
}

// TournamentMatch

func DeserializeTournamentMatches(pbMatches []*pb.TournamentMatch) ([]FullMatch, error) {
	matches := make([]FullMatch, 0, len(pbMatches))

	for _, pbMatch := range pbMatches {
		tournamentKey, err := uuid.Parse(pbMatch.TournamentKey)
		if err != nil {
			return nil, err
		}
		createdOn, err := time.Parse(time.RFC3339, pbMatch.CreatedOn)
		if err != nil {
			return nil, err
		}
		matches = append(matches, FullMatch{
			TournamentKey: tournamentKey,
			GameID:        GameID(pbMatch.GameId),
			WhiteID:       pbMatch.WhiteId,
			BlackID:       pbMatch.BlackId,
			Round:         int32(pbMatch.Round),
			CreatedOn:     createdOn,
		})
	}

	return matches, nil
}

// Game Input / Output

func MarshalGameOutputError(gameID GameID, messageID string, err error) ([]byte, error) {
	pbOutput := &pb.GameOutput{
		GameId:    gameID.String(),
		MessageId: messageID,
		Value: &pb.GameOutput_Error{
			Error: &pb.ErrorOutput{Message: err.Error()},
		},
	}
	return pbOutput.MarshalVT()
}

func MarshalGameOutputInit(gameID GameID, state *pb.ChessState, self *pb.PlayerState) ([]byte, error) {
	pbOutput := &pb.GameOutput{
		GameId: gameID.String(),
		Value: &pb.GameOutput_Init{
			Init: &pb.InitOutput{State: state, Self: self},
		},
	}
	return pbOutput.MarshalVT()
}

func SerializeGameOutputPlayers(gameID GameID, white, black *pb.PlayerState) *pb.GameOutput {
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

func SerializeGameOutputForfeit(gameID GameID, messageID string, endState EndKind) *pb.GameOutput {
	return &pb.GameOutput{
		GameId:    gameID.String(),
		MessageId: messageID,
		Value: &pb.GameOutput_Forfeit{
			Forfeit: &pb.ForfeitOutput{
				EndState: SerializeEndKind(endState),
			},
		},
	}
}

func SerializeGameOutputMove(gameID GameID, messageID string, move *pb.HistMove, game *pb.ChessGame, updatedAt time.Time) *pb.GameOutput {
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

func SerializeGameOutputChat(gameID GameID, messageID string, chat Chat) *pb.GameOutput {
	return &pb.GameOutput{
		GameId:    gameID.String(),
		MessageId: messageID,
		Value: &pb.GameOutput_Chat{Chat: &pb.ChatMessage{
			Player:  SerializePlayer(chat.Player),
			Message: chat.Message,
			SentAt:  chat.SentAt.Format(time.RFC3339),
		}},
	}
}

func SerializeGameOutputUndo(gameID GameID, messageID string, undoKind string, undoID int64, state *ChessState) *pb.GameOutput {
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
