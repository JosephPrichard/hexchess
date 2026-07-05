package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"hexchess-lib/enum"
	"hexchess-lib/optional"
	"hexchess-lib/serrors"
	"hexchess-svc/chess"
	"hexchess-svc/pb"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

// domain.PlayerState

func UnmarshalPlayer(bytes []byte) (PlayerState, error) {
	var pbPlayer pb.PlayerState
	if err := proto.Unmarshal(bytes, &pbPlayer); err != nil {
		return PlayerState{}, err
	}
	return PlayerState{ID: pbPlayer.Id, Name: pbPlayer.Name, Country: pbPlayer.Country, Present: true}, nil
}

func MarshalPlayer(player PlayerState) ([]byte, error) {
	pbPlayer := pb.PlayerState{
		Id: player.ID, Name: player.Name, Country: player.Country, IsGuest: IsGuestID(player.ID),
	}
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
		return nil, err
	}

	game, err := chess.DeserializeGame(pbChess.Game)
	if err != nil {
		return nil, serrors.Wrap("deserialize game", err)
	}
	initialBoard, err := chess.DeserializeBoard(pbChess.InitialBoard)
	if err != nil {
		return nil, serrors.Wrap("deserialize initial board", err, "board", pbChess.Game.Board)
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
		ID:           GameID(pbChess.Id),
		WhitePlayer:  DeserializePlayer(pbChess.WhitePlayer),
		BlackPlayer:  DeserializePlayer(pbChess.BlackPlayer),
		FirstColor:   firstColor,
		Mode:         mode,
	}, nil
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
	return proto.Marshal(&pb.ChatMessages{
		Chats: pbChats,
	})
}

func UnmarshalChat(bytes []byte) (Chat, error) {
	pbChat := &pb.ChatMessage{}
	if err := proto.Unmarshal(bytes, pbChat); err != nil {
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

// UserMessage

func MarshalUserMessage(pbUserMessage *pb.UserMessage) (Challenge, error) {
	switch message := (pbUserMessage.Value).(type) {
	case *pb.UserMessage_Challenge:
		challenge := message.Challenge

		madeOn, err := time.Parse(time.RFC3339, challenge.MadeOn)
		if err != nil {
			return Challenge{}, err
		}

		mode, modeErr := enum.Parse(challenge.Mode, GameModeEnums)
		startColor, colorErr := enum.Parse(challenge.StartColor, GameColorEnums)
		if err := errors.Join(modeErr, colorErr); err != nil {
			return Challenge{}, err
		}

		return Challenge{
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
		return Challenge{}, fmt.Errorf("unknown message type: %T", pbUserMessage)
	}
}

func MarshalUserMessageJson(pbUserMessage *pb.UserMessage) ([]byte, error) {
	challenge, err := MarshalUserMessage(pbUserMessage)
	if err != nil {
		return nil, err
	}
	return json.Marshal(challenge)
}

func SerializeChallengeMessage(challenge Challenge) *pb.UserMessage {
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

	mode, modeErr := enum.Parse(pbGameEvent.GameMode, GameModeEnums)
	replayResult, resultErr := enum.Parse(pbGameEvent.ReplayResult, ReplayResultEnums)
	replayCause, causeErr := enum.Parse(pbGameEvent.ReplayCause, ReplayCauseEnums)
	if err := errors.Join(modeErr, resultErr, causeErr); err != nil {
		return FinishedGame{}, err
	}

	board, err := chess.DeserializeBoard(pbGameEvent.Board)
	if err != nil {
		return FinishedGame{}, serrors.Wrap("deserialize board", err, "board", pbGameEvent.Board)
	}

	return FinishedGame{
		GameID:       GameID(pbGameEvent.GameId),
		Board:        board,
		Moves:        chess.DeserializeHistMoveList(pbGameEvent.Moves),
		WhitePlayer:  DeserializePlayer(pbGameEvent.WhitePlayer),
		BlackPlayer:  DeserializePlayer(pbGameEvent.BlackPlayer),
		ReplayMode:   mode,
		ReplayResult: replayResult,
		ReplayCause:  replayCause,
	}, nil
}

func MarshalFinishedGame(event FinishedGame) ([]byte, error) {
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

// StartGameEvent

func UnmarshalGameMetadataUpdt(bytes []byte) (GameMetadataUpdt, error) {
	var pbGameEvent pb.UpdtMetadataEvent
	if err := proto.Unmarshal(bytes, &pbGameEvent); err != nil {
		return GameMetadataUpdt{}, err
	}

	mode, modeErr := enum.Parse(pbGameEvent.Mode, GameModeEnums)
	firstColor, colorErr := enum.Parse(pbGameEvent.FirstColor, GameColorEnums)
	if err := errors.Join(modeErr, colorErr); err != nil {
		return GameMetadataUpdt{}, err
	}

	return GameMetadataUpdt{
		GameID:      GameID(pbGameEvent.GameId),
		WhitePlayer: optional.Maybe[int64]{Value: pbGameEvent.WhitePlayer, IsPresent: pbGameEvent.WhitePlayer >= 0},
		BlackPlayer: optional.Maybe[int64]{Value: pbGameEvent.BlackPlayer, IsPresent: pbGameEvent.BlackPlayer >= 0},
		FirstColor:  firstColor,
		Mode:        mode,
	}, nil
}

func MarshalGameMetadataUpdt(event GameMetadataUpdt) ([]byte, error) {
	return proto.Marshal(&pb.UpdtMetadataEvent{
		GameId:      event.GameID.String(),
		WhitePlayer: event.WhitePlayer.OrElse(-1),
		BlackPlayer: event.BlackPlayer.OrElse(-1),
		FirstColor:  event.FirstColor.String(),
		Mode:        event.Mode.String(),
	})
}

// MatchCreation

func UnmarshalMatchCreation(bytes []byte) ([]MatchCreation, error) {
	var pbMatches pb.MatchCreations
	if err := proto.Unmarshal(bytes, &pbMatches); err != nil {
		return nil, err
	}

	var matches []MatchCreation

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
	var pbMatches []*pb.MatchCreation
	for _, m := range matches {
		pbMatches = append(pbMatches, &pb.MatchCreation{
			GameId:  m.GameID.String(),
			Mode:    m.GameMode.String(),
			WhiteId: m.WhiteID,
			BlackId: m.BlackID,
		})
	}
	return proto.Marshal(&pb.MatchCreations{
		Creations: pbMatches,
	})
}

// AdvanceTournamentEvent

func UnmarshalAdvanceTournamentEvent(bytes []byte) (AdvanceTournamentEvent, error) {
	var pbEvent pb.AdvanceTournamentEvent
	if err := proto.Unmarshal(bytes, &pbEvent); err != nil {
		return AdvanceTournamentEvent{}, serrors.Wrap("unmarshal create scheduled tournament event", err)
	}
	tournamentKey, err := uuid.Parse(pbEvent.TournamentKey)
	if err != nil {
		return AdvanceTournamentEvent{}, serrors.Wrap("parse tournament key", err, "tournamentKey", pbEvent.TournamentKey)
	}
	eventID, err := uuid.Parse(pbEvent.EventId)
	if err != nil {
		return AdvanceTournamentEvent{}, serrors.Wrap("parse event id", err, "eventID", pbEvent.EventId)
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

func DeserializeParticipantOutput(pbParticipant *pb.TournamentOutput_Participant) (LbdUser, error) {
	if pbParticipant == nil || pbParticipant.Participant == nil {
		return LbdUser{}, nil
	}
	return DeserializeLbdUser(pbParticipant.Participant)
}

func DeserializeMatchmakingOutput(pbMatchmaking *pb.TournamentOutput_Matchmaking) ([]FullMatch, error) {
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
			return output, serrors.Wrap("deserialize participant output", err)
		}
		return TournamentOutput{Key: ParticipantKey, Value: TournamentOutput_Participant(lbdUser)}, nil
	case *pb.TournamentOutput_Countdown:
		return TournamentOutput{Key: CountdownKey, Value: TournamentOutput_Countdown{}}, nil
	case *pb.TournamentOutput_Start:
		return TournamentOutput{Key: StartKey, Value: TournamentOutput_Start{}}, nil
	case *pb.TournamentOutput_Matchmaking:
		matches, err := DeserializeMatchmakingOutput(pbOutputValue)
		if err != nil {
			return output, serrors.Wrap("deserialize matchmaking output", err)
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

func DeserializeTournamentMatches(pbMatches []*pb.TournamentMatch) ([]FullMatch, error) {
	var matches []FullMatch

	var serdeErrs []error

	for i, pbMatch := range pbMatches {
		tournamentKey, err := uuid.Parse(pbMatch.TournamentKey)
		if err != nil {
			serdeErrs = append(serdeErrs, serrors.Wrap("invalid tournament key", err, "i", i, "tournamentKey", pbMatch.TournamentKey))
			continue
		}
		createdOn, err := time.Parse(time.RFC3339, pbMatch.CreatedOn)
		if err != nil {
			serdeErrs = append(serdeErrs, serrors.Wrap("parse created on", err, "i", i, "createdOn", pbMatch.CreatedOn))
			continue
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

	return matches, errors.Join(serdeErrs...)
}
