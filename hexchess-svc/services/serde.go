package svc

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"hexchess-svc/internal/enum"
	"time"

	"google.golang.org/protobuf/proto"

	"hexchess-svc/chess"
	"hexchess-svc/pb"
)

// PlayerState

func UnmarshalPlayer(bytes []byte) (PlayerState, error) {
	var pbPlayer pb.PlayerState
	if err := proto.Unmarshal(bytes, &pbPlayer); err != nil {
		return PlayerState{}, fmt.Errorf("unmarshal player: %w", err)
	}
	return PlayerState{ID: pbPlayer.Id, Name: pbPlayer.Name, Country: pbPlayer.Country, Present: true}, nil
}

func MarshalPlayer(player PlayerState) ([]byte, error) {
	pbPlayer := pb.PlayerState{Id: player.ID, Name: player.Name, Country: player.Country, IsGuest: IsGuestID(player.ID)}
	return proto.Marshal(&pbPlayer)
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

// ChessState

func UnmarshalChessState(bytes []byte) (*ChessState, error) {
	var pbChess pb.ChessState
	if err := proto.Unmarshal(bytes, &pbChess); err != nil {
		return nil, fmt.Errorf("unmarshal chess state: %w", err)
	}

	game, err := chess.DeserializeGame(pbChess.Game)
	if err != nil {
		return nil, fmt.Errorf("deserialize game: %w", err)
	}
	initialBoard, err := chess.DeserializeBoard(pbChess.InitialBoard)
	if err != nil {
		return nil, fmt.Errorf("deserialize initial board %v: %w", pbChess.Game.Board, err)
	}

	mode, modeErr := enum.Parse(pbChess.Mode, GameModeEnums)
	firstColor, colorErr := enum.Parse(pbChess.FirstColor, GameColorEnums)
	if err := errors.Join(modeErr, colorErr); err != nil {
		return nil, err
	}

	return &ChessState{
		Game:         game,
		InitialBoard: initialBoard,
		UndoState:    UndoState{UndoID: pbChess.UndoId},
		EndState:     DeserializeEndKind(pbChess.EndState),
		ChessMeta: ChessMeta{
			ID:          pbChess.Id,
			WhitePlayer: DeserializePlayer(pbChess.WhitePlayer),
			BlackPlayer: DeserializePlayer(pbChess.BlackPlayer),
			FirstColor:  firstColor,
			Mode:        mode,
			Touch:       time.UnixMilli(pbChess.Touch),
		},
	}, nil
}

func SerializeChessState(state *ChessState) *pb.ChessState {
	if state == nil {
		return nil
	}
	return &pb.ChessState{
		Id:           state.ID,
		Game:         chess.SerializeGame(&state.Game),
		WhitePlayer:  SerializePlayer(state.WhitePlayer),
		BlackPlayer:  SerializePlayer(state.BlackPlayer),
		FirstColor:   state.FirstColor.String(),
		Mode:         state.Mode.String(),
		Touch:        state.Touch.UnixMilli(),
		InitialBoard: chess.SerializeBoard(&state.InitialBoard),
		UndoId:       state.UndoID,
		EndState:     SerializeEndKind(state.EndState),
	}
}

// ChessMeta

func UnmarshalChessMeta(bytes []byte) (ChessMeta, error) {
	var pbChess pb.ChessState
	if err := proto.Unmarshal(bytes, &pbChess); err != nil {
		return ChessMeta{}, fmt.Errorf("unmarshal chess meta: %w", err)
	}

	mode, modeErr := enum.Parse(pbChess.Mode, GameModeEnums)
	firstColor, colorErr := enum.Parse(pbChess.FirstColor, GameColorEnums)
	if err := errors.Join(modeErr, colorErr); err != nil {
		return ChessMeta{}, err
	}

	return ChessMeta{
		ID:          pbChess.Id,
		WhitePlayer: DeserializePlayer(pbChess.WhitePlayer),
		BlackPlayer: DeserializePlayer(pbChess.BlackPlayer),
		FirstColor:  firstColor,
		Mode:        mode,
	}, nil
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

// UserMessage

func MarshalUserMessageJson(pbUserMessage *pb.UserMessage) ([]byte, error) {
	switch message := (pbUserMessage.Value).(type) {
	case *pb.UserMessage_Challenge:
		challenge := message.Challenge
		madeOn, err := time.Parse(time.RFC3339, challenge.MadeOn)
		if err != nil {
			return nil, fmt.Errorf("parse challenge made on: %w", err)
		}

		mode, modeErr := enum.Parse(challenge.Mode, GameModeEnums)
		startColor, colorErr := enum.Parse(challenge.StartColor, GameColorEnums)
		if err := errors.Join(modeErr, colorErr); err != nil {
			return nil, err
		}

		return json.Marshal(ChallengeDTO{
			ChallengerID:      challenge.ChallengerId,
			ChallengerName:    challenge.ChallengerName,
			ChallengerCountry: challenge.ChallengerCountry,
			ChallengerElo:     challenge.ChallengerElo,
			ChallengeeID:      challenge.ChallengeeId,
			ChallengeeName:    challenge.ChallengeeName,
			ChallengeeCountry: challenge.ChallengeeCountry,
			ChallengeeElo:     challenge.ChallengeeElo,
			StartColor:        startColor,
			Mode:              mode,
			MadeOn:            madeOn,
		})
	default:
		return nil, fmt.Errorf("unknown message type: %T", pbUserMessage)
	}
}

func SerializeChallengeMessage(challenge ChallengeDTO) *pb.UserMessage {
	challengeMessage := &pb.UserMessage_Challenge{
		Challenge: &pb.ChallengeMessage{
			ChallengerId:      challenge.ChallengerID,
			ChallengerName:    challenge.ChallengerName,
			ChallengerCountry: challenge.ChallengerCountry,
			ChallengerElo:     challenge.ChallengerElo,
			ChallengeeId:      challenge.ChallengeeID,
			ChallengeeName:    challenge.ChallengeeName,
			ChallengeeCountry: challenge.ChallengeeCountry,
			ChallengeeElo:     challenge.ChallengeeElo,
			Mode:              challenge.Mode.String(),
			StartColor:        challenge.StartColor.String(),
			MadeOn:            challenge.MadeOn.Format(time.RFC3339),
		},
	}
	return &pb.UserMessage{UserId: challenge.ChallengeeID, Value: challengeMessage}
}

// FinishGameEvent

func UnmarshalFinishGameEvent(bytes []byte) (FinishGameEvent, error) {
	var pbGameEvent pb.FinishGameEvent
	if err := proto.Unmarshal(bytes, &pbGameEvent); err != nil {
		return FinishGameEvent{}, fmt.Errorf("unmarshal finish game event: %w", err)
	}

	mode, modeErr := enum.Parse(pbGameEvent.GameMode, GameModeEnums)
	replayResult, resultErr := enum.Parse(pbGameEvent.ReplayResult, ReplayResultEnums)
	replayCause, causeErr := enum.Parse(pbGameEvent.ReplayCause, ReplayCauseEnums)
	if err := errors.Join(modeErr, resultErr, causeErr); err != nil {
		return FinishGameEvent{}, err
	}

	gameID, err := uuid.Parse(pbGameEvent.GameId)
	if err != nil {
		return FinishGameEvent{}, fmt.Errorf("parse game uuid: %w", err)
	}

	board, err := chess.DeserializeBoard(pbGameEvent.Board)
	if err != nil {
		return FinishGameEvent{}, fmt.Errorf("deserialize board %v: %w", pbGameEvent.Board, err)
	}

	return FinishGameEvent{
		GameID:       gameID,
		Board:        board,
		Moves:        chess.DeserializeHistMoveList(pbGameEvent.Moves),
		WhitePlayer:  DeserializePlayer(pbGameEvent.WhitePlayer),
		BlackPlayer:  DeserializePlayer(pbGameEvent.BlackPlayer),
		ReplayMode:   mode,
		ReplayResult: replayResult,
		ReplayCause:  replayCause,
	}, nil
}

func MarshalFinishGameEvent(event FinishGameEvent) ([]byte, error) {
	return proto.Marshal(&pb.FinishGameEvent{
		GameId:       event.GameID.String(),
		Board:        chess.SerializeBoard(&event.Board),
		Moves:        chess.SerializeMoveList(event.Moves),
		WhitePlayer:  SerializePlayer(event.WhitePlayer),
		BlackPlayer:  SerializePlayer(event.BlackPlayer),
		GameMode:     event.ReplayMode.String(),
		ReplayResult: event.ReplayResult.String(),
		ReplayCause:  event.ReplayCause.String(),
	})
}

// AdvanceTournamentEvent

func MarshalAdvanceTournamentEvent(tournamentKey uuid.UUID) ([]byte, error) {
	return proto.Marshal(&pb.AdvanceTournamentEvent{
		TournamentKey: tournamentKey.String(),
	})
}

// ReplayUsersDTO

func SerializeReplayOutput(gameID uuid.UUID, replay FullReplayDTO) *pb.GameOutput {
	return &pb.GameOutput{
		GameId: gameID.String(),
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
