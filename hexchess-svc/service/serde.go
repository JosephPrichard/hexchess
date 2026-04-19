package svc

import (
	"encoding/json"
	"errors"
	"fmt"
	"hexchess-svc/domain"
	"hexchess-svc/util/enum"
	"time"

	"github.com/google/uuid"

	"google.golang.org/protobuf/proto"

	"hexchess-svc/chess"
	"hexchess-svc/pb"
)

// domain.PlayerState

func UnmarshalPlayer(bytes []byte) (domain.PlayerState, error) {
	var pbPlayer pb.PlayerState
	if err := proto.Unmarshal(bytes, &pbPlayer); err != nil {
		return domain.PlayerState{}, err
	}
	return domain.PlayerState{ID: pbPlayer.Id, Name: pbPlayer.Name, Country: pbPlayer.Country, Present: true}, nil
}

func MarshalPlayer(player domain.PlayerState) ([]byte, error) {
	pbPlayer := pb.PlayerState{
		Id: player.ID, Name: player.Name, Country: player.Country, IsGuest: domain.IsGuestID(player.ID),
	}
	return proto.Marshal(&pbPlayer)
}

func DeserializePlayer(pbPlayer *pb.PlayerState) domain.PlayerState {
	var player domain.PlayerState
	if pbPlayer != nil {
		player = domain.PlayerState{ID: pbPlayer.Id, Name: pbPlayer.Name, Country: pbPlayer.Country, Present: true}
	}
	return player
}

func SerializePlayer(player domain.PlayerState) *pb.PlayerState {
	if player.Present {
		return &pb.PlayerState{Id: player.ID, Name: player.Name, Country: player.Country, IsGuest: domain.IsGuestID(player.ID)}
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
		return nil, err
	}

	game, err := chess.DeserializeGame(pbChess.Game)
	if err != nil {
		return nil, fmt.Errorf("deserialize game: %w", err)
	}
	initialBoard, err := chess.DeserializeBoard(pbChess.InitialBoard)
	if err != nil {
		return nil, fmt.Errorf("deserialize initial board %v: %w", pbChess.Game.Board, err)
	}

	mode, modeErr := enum.ParseWithErr(pbChess.Mode, domain.GameModeEnums)
	firstColor, colorErr := enum.ParseWithErr(pbChess.FirstColor, domain.GameColorEnums)

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
		return ChessMeta{}, err
	}

	mode, modeErr := enum.ParseWithErr(pbChess.Mode, domain.GameModeEnums)
	firstColor, colorErr := enum.ParseWithErr(pbChess.FirstColor, domain.GameColorEnums)
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
			return nil, err
		}

		mode, modeErr := enum.ParseWithErr(challenge.Mode, domain.GameModeEnums)
		startColor, colorErr := enum.ParseWithErr(challenge.StartColor, domain.GameColorEnums)
		if err := errors.Join(modeErr, colorErr); err != nil {
			return nil, err
		}

		return json.Marshal(domain.Challenge{
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

func SerializeChallengeMessage(challenge domain.Challenge) *pb.UserMessage {
	challengeMessage := &pb.UserMessage_Challenge{
		Challenge: &pb.Challenge{
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
		return FinishGameEvent{}, err
	}

	mode, modeErr := enum.ParseWithErr(pbGameEvent.GameMode, domain.GameModeEnums)
	replayResult, resultErr := enum.ParseWithErr(pbGameEvent.ReplayResult, domain.ReplayResultEnums)
	replayCause, causeErr := enum.ParseWithErr(pbGameEvent.ReplayCause, domain.ReplayCauseEnums)
	if err := errors.Join(modeErr, resultErr, causeErr); err != nil {
		return FinishGameEvent{}, err
	}

	board, err := chess.DeserializeBoard(pbGameEvent.Board)
	if err != nil {
		return FinishGameEvent{}, fmt.Errorf("deserialize board %v: %w", pbGameEvent.Board, err)
	}

	return FinishGameEvent{
		GameID:       pbGameEvent.GameId,
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
		GameId:       event.GameID,
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

func SerializeCreateTournamentMatchesEvent(event CreateTournamentMatchesEvent) *pb.CreateTournamentMatchesEvent {
	var pbMatches []*pb.CreateTournamentMatch
	for _, match := range event.Matches {
		pbMatches = append(pbMatches, &pb.CreateTournamentMatch{
			GameId:   match.GameID,
			WhiteId:  match.WhiteID,
			BlackId:  match.BlackID,
			GameMode: match.GameMode.String(),
		})
	}
	return &pb.CreateTournamentMatchesEvent{
		TournamentKey: event.TournamentKey.String(),
		Matches:       pbMatches,
	}
}

func UnmarshalCreateTournamentMatchesEvent(bytes []byte) (e CreateTournamentMatchesEvent, err error) {
	var pbEvent pb.CreateTournamentMatchesEvent
	if err := proto.Unmarshal(bytes, &pbEvent); err != nil {
		return e, err
	}

	var serdeErrs []error

	tournamentKey, err := uuid.Parse(pbEvent.TournamentKey)
	if err != nil {
		serdeErrs = append(serdeErrs, err)
	}
	var matches []TournamentMatchCreation
	for _, pbMatch := range pbEvent.Matches {
		mode, err := enum.ParseWithErr(pbMatch.GameMode, domain.GameModeEnums)
		if err != nil {
			serdeErrs = append(serdeErrs, err)
			continue
		}
		matches = append(matches, TournamentMatchCreation{
			GameID:   pbMatch.GameId,
			WhiteID:  pbMatch.WhiteId,
			BlackID:  pbMatch.BlackId,
			GameMode: mode,
		})
	}
	return CreateTournamentMatchesEvent{TournamentKey: tournamentKey, Matches: matches}, errors.Join(serdeErrs...)
}

// Replay

func SerializeReplayOutput(gameID string, replay domain.FullReplay) *pb.GameOutput {
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

func SerializeParticipantOutput(tournamentKey uuid.UUID, lbdUser domain.LbdUser) *pb.TournamentOutput {
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

func DeserializeParticipantOutput(pbParticipant *pb.TournamentOutput_Participant) (domain.LbdUser, error) {
	if pbParticipant == nil || pbParticipant.Participant == nil {
		return domain.LbdUser{}, nil
	}
	return DeserializeLbdUser(pbParticipant.Participant)
}

func DeserializeMatchmakingOutput(pbMatchmaking *pb.TournamentOutput_Matchmaking) ([]domain.Match, error) {
	if pbMatchmaking == nil || pbMatchmaking.Matchmaking == nil {
		return nil, nil
	}
	return DeserializeTournamentMatches(pbMatchmaking.Matchmaking.Matches)
}

func MarshalTournamentOutputJson(pbOutput *pb.TournamentOutput) ([]byte, error) {
	if pbOutput == nil || pbOutput.Value == nil {
		return []byte{}, nil
	}

	var key TournamentOutputKey
	var value isTournamentOutput_Value

	switch pbOutputValue := pbOutput.Value.(type) {
	case *pb.TournamentOutput_Participant:
		lbdUser, err := DeserializeParticipantOutput(pbOutputValue)
		if err != nil {
			return nil, err
		}
		key = ParticipantKey
		value = TournamentOutput_Participant(lbdUser)
	case *pb.TournamentOutput_Countdown:
		key = CountdownKey
		value = TournamentOutput_Countdown{}
	case *pb.TournamentOutput_Start:
		key = StartKey
		value = TournamentOutput_Start{}
	case *pb.TournamentOutput_Matchmaking:
		matches, err := DeserializeMatchmakingOutput(pbOutputValue)
		if err != nil {
			return nil, err
		}
		key = MatchmakingKey
		value = TournamentOutput_Matchmaking{Matches: matches}
	case *pb.TournamentOutput_Error:
		key = ErrorKey
		value = TournamentOutput_Error(pbOutputValue.Error.Message)
	}

	return json.Marshal(TournamentOutput{Key: key, Value: value})
}

// LbdUser

func SerializeLbdUser(user domain.LbdUser) *pb.LbdUser {
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

func DeserializeLbdUser(user *pb.LbdUser) (domain.LbdUser, error) {
	joinedOn, err := time.Parse(time.RFC3339, user.JoinedOn)
	if err != nil {
		return domain.LbdUser{}, err
	}

	return domain.LbdUser{
		User: domain.User{
			ID:       user.Id,
			Username: user.Username,
			Country:  user.Country,
			Bio:      user.Bio,
			JoinedOn: joinedOn,
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

func SerializeTournamentMatches(matches []domain.Match) []*pb.TournamentMatch {
	var pbMatches []*pb.TournamentMatch

	for _, match := range matches {
		pbMatches = append(pbMatches, &pb.TournamentMatch{
			TournamentKey: match.TournamentKey.String(),
			GameId:        match.GameID,
			WhiteId:       match.WhiteID,
			BlackId:       match.BlackID,
			Round:         int64(match.Round),
			CreatedOn:     match.CreatedOn.Format(time.RFC3339),
		})
	}

	return pbMatches
}

func DeserializeTournamentMatches(pbMatches []*pb.TournamentMatch) ([]domain.Match, error) {
	var matches []domain.Match

	var serdeErrs []error

	for i, pbMatch := range pbMatches {
		tournamentKey, err := uuid.Parse(pbMatch.TournamentKey)
		if err != nil {
			serdeErrs = append(serdeErrs, fmt.Errorf("match %d: invalid tournament key: %w", i, err))
			continue
		}
		createdOn, err := time.Parse(time.RFC3339, pbMatch.CreatedOn)
		if err != nil {
			serdeErrs = append(serdeErrs, fmt.Errorf("match %d: parse created on: %w", i, err))
			continue
		}
		matches = append(matches, domain.Match{
			TournamentKey: tournamentKey,
			GameID:        pbMatch.GameId,
			WhiteID:       pbMatch.WhiteId,
			BlackID:       pbMatch.BlackId,
			Round:         int32(pbMatch.Round),
			CreatedOn:     createdOn,
		})
	}

	return matches, errors.Join(serdeErrs...)
}
