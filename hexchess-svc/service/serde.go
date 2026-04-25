package svc

import (
	"encoding/json"
	"errors"
	"fmt"
	"hexchess-svc/model"
	"hexchess-svc/util/enum"
	"time"

	"github.com/google/uuid"

	"google.golang.org/protobuf/proto"

	"hexchess-svc/hexchess"
	"hexchess-svc/pb"
)

// domain.PlayerState

func UnmarshalPlayer(bytes []byte) (model.PlayerState, error) {
	var pbPlayer pb.PlayerState
	if err := proto.Unmarshal(bytes, &pbPlayer); err != nil {
		return model.PlayerState{}, err
	}
	return model.PlayerState{ID: pbPlayer.Id, Name: pbPlayer.Name, Country: pbPlayer.Country, Present: true}, nil
}

func MarshalPlayer(player model.PlayerState) ([]byte, error) {
	pbPlayer := pb.PlayerState{
		Id: player.ID, Name: player.Name, Country: player.Country, IsGuest: model.IsGuestID(player.ID),
	}
	return proto.Marshal(&pbPlayer)
}

func DeserializePlayer(pbPlayer *pb.PlayerState) model.PlayerState {
	var player model.PlayerState
	if pbPlayer != nil {
		player = model.PlayerState{ID: pbPlayer.Id, Name: pbPlayer.Name, Country: pbPlayer.Country, Present: true}
	}
	return player
}

func SerializePlayer(player model.PlayerState) *pb.PlayerState {
	if player.Present {
		return &pb.PlayerState{Id: player.ID, Name: player.Name, Country: player.Country, IsGuest: model.IsGuestID(player.ID)}
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

	game, err := hexchess.DeserializeGame(pbChess.Game)
	if err != nil {
		return nil, fmt.Errorf("deserialize game: %w", err)
	}
	initialBoard, err := hexchess.DeserializeBoard(pbChess.InitialBoard)
	if err != nil {
		return nil, fmt.Errorf("deserialize initial board %v: %w", pbChess.Game.Board, err)
	}

	mode, modeErr := enum.ParseWithErr(pbChess.Mode, model.GameModeEnums)
	firstColor, colorErr := enum.ParseWithErr(pbChess.FirstColor, model.GameColorEnums)

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
		Game:         hexchess.SerializeGame(&state.Game),
		WhitePlayer:  SerializePlayer(state.WhitePlayer),
		BlackPlayer:  SerializePlayer(state.BlackPlayer),
		FirstColor:   state.FirstColor.String(),
		Mode:         state.Mode.String(),
		Touch:        state.Touch.UnixMilli(),
		InitialBoard: hexchess.SerializeBoard(&state.InitialBoard),
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

	mode, modeErr := enum.ParseWithErr(pbChess.Mode, model.GameModeEnums)
	firstColor, colorErr := enum.ParseWithErr(pbChess.FirstColor, model.GameColorEnums)
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

func MarshalUserMessage(pbUserMessage *pb.UserMessage) (model.Challenge, error) {
	switch message := (pbUserMessage.Value).(type) {
	case *pb.UserMessage_Challenge:
		challenge := message.Challenge

		madeOn, err := time.Parse(time.RFC3339, challenge.MadeOn)
		if err != nil {
			return model.Challenge{}, err
		}

		mode, modeErr := enum.ParseWithErr(challenge.Mode, model.GameModeEnums)
		startColor, colorErr := enum.ParseWithErr(challenge.StartColor, model.GameColorEnums)
		if err := errors.Join(modeErr, colorErr); err != nil {
			return model.Challenge{}, err
		}

		return model.Challenge{
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
		}, nil
	default:
		return model.Challenge{}, fmt.Errorf("unknown message type: %T", pbUserMessage)
	}
}

func MarshalUserMessageJson(pbUserMessage *pb.UserMessage) ([]byte, error) {
	challenge, err := MarshalUserMessage(pbUserMessage)
	if err != nil {
		return nil, err
	}
	return json.Marshal(challenge)
}

func SerializeChallengeMessage(challenge model.Challenge) *pb.UserMessage {
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

func UnmarshalFinishedGame(bytes []byte) (FinishedGame, error) {
	var pbGameEvent pb.FinishGameEvent
	if err := proto.Unmarshal(bytes, &pbGameEvent); err != nil {
		return FinishedGame{}, err
	}

	mode, modeErr := enum.ParseWithErr(pbGameEvent.GameMode, model.GameModeEnums)
	replayResult, resultErr := enum.ParseWithErr(pbGameEvent.ReplayResult, model.ReplayResultEnums)
	replayCause, causeErr := enum.ParseWithErr(pbGameEvent.ReplayCause, model.ReplayCauseEnums)
	if err := errors.Join(modeErr, resultErr, causeErr); err != nil {
		return FinishedGame{}, err
	}

	board, err := hexchess.DeserializeBoard(pbGameEvent.Board)
	if err != nil {
		return FinishedGame{}, fmt.Errorf("deserialize board %v: %w", pbGameEvent.Board, err)
	}

	return FinishedGame{
		GameID:       pbGameEvent.GameId,
		Board:        board,
		Moves:        hexchess.DeserializeHistMoveList(pbGameEvent.Moves),
		WhitePlayer:  DeserializePlayer(pbGameEvent.WhitePlayer),
		BlackPlayer:  DeserializePlayer(pbGameEvent.BlackPlayer),
		ReplayMode:   mode,
		ReplayResult: replayResult,
		ReplayCause:  replayCause,
	}, nil
}

func MarshalFinishedGame(event FinishedGame) ([]byte, error) {
	return proto.Marshal(&pb.FinishGameEvent{
		GameId:       event.GameID,
		Board:        hexchess.SerializeBoard(&event.Board),
		Moves:        hexchess.SerializeMoveList(event.Moves),
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
		mode, err := enum.ParseWithErr(pbMatch.GameMode, model.GameModeEnums)
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

func SerializeReplayOutput(gameID string, replay model.FullReplay) *pb.GameOutput {
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

func SerializeParticipantOutput(tournamentKey uuid.UUID, lbdUser model.LbdUser) *pb.TournamentOutput {
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

func DeserializeParticipantOutput(pbParticipant *pb.TournamentOutput_Participant) (model.LbdUser, error) {
	if pbParticipant == nil || pbParticipant.Participant == nil {
		return model.LbdUser{}, nil
	}
	return DeserializeLbdUser(pbParticipant.Participant)
}

func DeserializeMatchmakingOutput(pbMatchmaking *pb.TournamentOutput_Matchmaking) ([]model.Match, error) {
	if pbMatchmaking == nil || pbMatchmaking.Matchmaking == nil {
		return nil, nil
	}
	return DeserializeTournamentMatches(pbMatchmaking.Matchmaking.Matches)
}

func DeserializeErrorOutput(pbError *pb.TournamentOutput_Error) string {
	if pbError == nil || pbError.Error == nil {
		return ""
	}
	return pbError.Error.Message
}

func MarshalTournamentOutput(pbOutput *pb.TournamentOutput) (output TournamentOutput, err error) {
	if pbOutput != nil && pbOutput.Value != nil {
		return output, nil
	}

	switch pbOutputValue := pbOutput.Value.(type) {
	case *pb.TournamentOutput_Participant:
		lbdUser, err := DeserializeParticipantOutput(pbOutputValue)
		if err != nil {
			return output, fmt.Errorf("deserialize participant output: %w", err)
		}
		return TournamentOutput{Key: ParticipantKey, Value: TournamentOutput_Participant(lbdUser)}, nil
	case *pb.TournamentOutput_Countdown:
		return TournamentOutput{Key: CountdownKey, Value: TournamentOutput_Countdown{}}, nil
	case *pb.TournamentOutput_Start:
		return TournamentOutput{Key: StartKey, Value: TournamentOutput_Start{}}, nil
	case *pb.TournamentOutput_Matchmaking:
		matches, err := DeserializeMatchmakingOutput(pbOutputValue)
		if err != nil {
			return output, fmt.Errorf("deserialize matchmaking output: %w", err)
		}
		return TournamentOutput{Key: MatchmakingKey, Value: TournamentOutput_Matchmaking{Matches: matches}}, nil
	case *pb.TournamentOutput_Error:
		return TournamentOutput{
			Key:   ErrorKey,
			Value: TournamentOutput_Error(DeserializeErrorOutput(pbOutputValue)),
		}, nil
	default:
		return output, fmt.Errorf("unknown tournament output type: %T", pbOutputValue)
	}
}

func MarshalTournamentOutputJson(pbOutput *pb.TournamentOutput) ([]byte, error) {
	output, err := MarshalTournamentOutput(pbOutput)
	if err != nil {
		return nil, err
	}
	return json.Marshal(output)
}

// LbdUser

func SerializeLbdUser(user model.LbdUser) *pb.LbdUser {
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

func DeserializeLbdUser(user *pb.LbdUser) (model.LbdUser, error) {
	joinedOn, err := time.Parse(time.RFC3339, user.JoinedOn)
	if err != nil {
		return model.LbdUser{}, err
	}

	return model.LbdUser{
		User: model.User{
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

func SerializeTournamentMatches(matches []model.Match) []*pb.TournamentMatch {
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

func DeserializeTournamentMatches(pbMatches []*pb.TournamentMatch) ([]model.Match, error) {
	var matches []model.Match

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
		matches = append(matches, model.Match{
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
