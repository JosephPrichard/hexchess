package svc

import (
	"encoding/json"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	"hexchess-svc/chess"
	"hexchess-svc/pb"
)

// PlayerState

func UnmarshalPlayer(b []byte) (PlayerState, error) {
	var pbPlayer pb.PlayerState
	if err := proto.Unmarshal(b, &pbPlayer); err != nil {
		return PlayerState{}, err
	}
	player := PlayerState{ID: pbPlayer.Id, Name: pbPlayer.Name, Country: pbPlayer.Country, IsGuest: pbPlayer.IsGuest, Present: true}
	return player, nil
}

func MarshalPlayer(p PlayerState) ([]byte, error) {
	pbPlayer := pb.PlayerState{Id: p.ID, Name: p.Name, Country: p.Country, IsGuest: p.IsGuest}
	return proto.Marshal(&pbPlayer)
}

func DeserializePlayer(pbPlayer *pb.PlayerState) PlayerState {
	var player PlayerState
	if pbPlayer != nil {
		player = PlayerState{ID: pbPlayer.Id, Name: pbPlayer.Name, Country: pbPlayer.Country, IsGuest: pbPlayer.IsGuest, Present: true}
	}
	return player
}

func SerializePlayer(p PlayerState) *pb.PlayerState {
	if p.Present {
		return &pb.PlayerState{Id: p.ID, Name: p.Name, Country: p.Country, IsGuest: p.IsGuest}
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

func UnmarshalChessState(b []byte) (st ChessState, err error) {
	var pbChess pb.ChessState
	if err := proto.Unmarshal(b, &pbChess); err != nil {
		return st, err
	}
	game, err := chess.DeserializeGame(pbChess.Game)
	if err != nil {
		return st, fmt.Errorf("deserialize game: %w", err)
	}
	initialBoard, err := chess.DeserializeBoard(pbChess.InitialBoard)
	if err != nil {
		return st, fmt.Errorf("deserialize initial board %v: %w", pbChess.Game.Board, err)
	}

	p := EnumParser{}
	color := p.Color(pbChess.FirstColor)
	mode := p.GameMode(pbChess.Mode)

	if err := p.Err(); err != nil {
		return st, err
	}

	return ChessState{
		Game:         game,
		InitialBoard: initialBoard,
		UndoState:    UndoState{UndoID: pbChess.UndoId},
		EndState:     DeserializeEndKind(pbChess.EndState),
		ChessMeta: ChessMeta{
			ID:          pbChess.Id,
			WhitePlayer: DeserializePlayer(pbChess.WhitePlayer),
			BlackPlayer: DeserializePlayer(pbChess.BlackPlayer),
			FirstColor:  color,
			Mode:        mode,
			Touch:       time.UnixMilli(pbChess.Touch),
		},
	}, nil
}

func SerializeChessState(s *ChessState) *pb.ChessState {
	if s == nil {
		return nil
	}
	return &pb.ChessState{
		Id:           s.ID,
		Game:         chess.SerializeGame(&s.Game),
		WhitePlayer:  SerializePlayer(s.WhitePlayer),
		BlackPlayer:  SerializePlayer(s.BlackPlayer),
		FirstColor:   s.FirstColor.String(),
		Mode:         s.Mode.String(),
		Touch:        s.Touch.UnixMilli(),
		InitialBoard: chess.SerializeBoard(&s.InitialBoard),
		UndoId:       s.UndoID,
		EndState:     SerializeEndKind(s.EndState),
	}
}

// ChessMeta

func UnmarshalChessMeta(b []byte) (m ChessMeta, err error) {
	var pbChess pb.ChessState
	if err := proto.Unmarshal(b, &pbChess); err != nil {
		return m, fmt.Errorf("unmarshal chess s: %w", err)
	}

	p := EnumParser{}
	color := p.Color(pbChess.FirstColor)
	mode := p.GameMode(pbChess.Mode)

	if err := p.Err(); err != nil {
		return m, err
	}

	return ChessMeta{
		ID:          pbChess.Id,
		WhitePlayer: DeserializePlayer(pbChess.WhitePlayer),
		BlackPlayer: DeserializePlayer(pbChess.BlackPlayer),
		FirstColor:  color,
		Mode:        mode,
	}, nil
}

// ChatMsg

func UnmarshalChat(b []byte) (chat StateChat, err error) {
	var pbChat pb.ChatMsg
	if err := proto.Unmarshal(b, &pbChat); err != nil {
		return chat, fmt.Errorf("marshal chat: %w", err)
	}
	sentAt, err := time.Parse(time.RFC3339, pbChat.SentAt)
	if err != nil {
		return chat, fmt.Errorf("parse chat sent at %s: %w", pbChat.SentAt, err)
	}
	return StateChat{
		Player:  DeserializePlayer(pbChat.Player),
		Message: pbChat.Message,
		SentAt:  sentAt,
	}, nil
}

func SerializeChat(chat StateChat) *pb.ChatMsg {
	return &pb.ChatMsg{
		Player:  SerializePlayer(chat.Player),
		Message: chat.Message,
		SentAt:  chat.SentAt.Format(time.RFC3339),
	}
}

func SerializeChats(chats []StateChat) []*pb.ChatMsg {
	pbChats := make([]*pb.ChatMsg, 0, len(chats))
	for _, chat := range chats {
		pbChats = append(pbChats, SerializeChat(chat))
	}
	return pbChats
}

// UserMsg

func MarshalUserMessageJson(pbUserMessage *pb.UserMessage) ([]byte, error) {
	switch message := (pbUserMessage.Value).(type) {
	case *pb.UserMessage_Challenge:
		challenge := message.Challenge
		madeOn, err := time.Parse(time.RFC3339, challenge.MadeOn)
		if err != nil {
			return nil, fmt.Errorf("parse challenge made on: %w", err)
		}
		return json.Marshal(ChallengeEntity{
			ChallengerID:      challenge.ChallengerId,
			ChallengerName:    challenge.ChallengerName,
			ChallengerCountry: challenge.ChallengerCountry,
			ChallengerElo:     challenge.ChallengerElo,
			ChallengeeID:      challenge.ChallengeeId,
			ChallengeeName:    challenge.ChallengeeName,
			ChallengeeCountry: challenge.ChallengeeCountry,
			ChallengeeElo:     challenge.ChallengeeElo,
			StartColor:        challenge.StartColor,
			Mode:              challenge.Mode,
			MadeOn:            madeOn,
		})
	default:
		return nil, fmt.Errorf("unknown message type: %T", pbUserMessage)
	}
}

func SerializeChallengeMessage(challenge ChallengeEntity) *pb.UserMessage {
	userChallengeMessage := &pb.UserMessage_Challenge{
		Challenge: &pb.ChallengeMessage{
			ChallengerId:      challenge.ChallengerID,
			ChallengerName:    challenge.ChallengerName,
			ChallengerCountry: challenge.ChallengerCountry,
			ChallengerElo:     challenge.ChallengerElo,
			ChallengeeId:      challenge.ChallengeeID,
			ChallengeeName:    challenge.ChallengeeName,
			ChallengeeCountry: challenge.ChallengeeCountry,
			ChallengeeElo:     challenge.ChallengeeElo,
			Mode:              challenge.Mode,
			StartColor:        challenge.StartColor,
			MadeOn:            challenge.MadeOn.Format(time.RFC3339),
		},
	}
	return &pb.UserMessage{UserId: challenge.ChallengeeID, Value: userChallengeMessage}
}

// FinishGameEvent

func UnmarshalFinishGameEvent(b []byte) (event FinishGameEvent, err error) {
	var pbGameEvent pb.FinishGameEvent
	if err := proto.Unmarshal(b, &pbGameEvent); err != nil {
		return event, fmt.Errorf("unmarshal finish game event: %w", err)
	}

	board, err := chess.DeserializeBoard(pbGameEvent.Board)
	if err != nil {
		return event, fmt.Errorf("deserialize board %v: %w", pbGameEvent.Board, err)
	}
	moves, err := chess.DeserializeHistMoveList(pbGameEvent.Moves)
	if err != nil {
		return event, fmt.Errorf("deserialize moves: %w", err)
	}

	p := EnumParser{}
	mode := p.GameMode(pbGameEvent.GameMode)
	replayResult := p.ReplayResult(pbGameEvent.ReplayResult)
	replayCause := p.ReplayCause(pbGameEvent.ReplayCause)

	if err := p.Err(); err != nil {
		return event, err
	}

	return FinishGameEvent{
		GameID:       pbGameEvent.GameId,
		Board:        board,
		Moves:        moves,
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

// Replay

func SerializeReplayOutput(gameID string, replay ReplayEntity) *pb.GameOutput {
	return &pb.GameOutput{
		GameId: gameID,
		Value: &pb.GameOutput_Replay{Replay: &pb.ReplayOutput{Replay: &pb.ReplayEntity{
			Id:           replay.ID,
			WhiteId:      replay.WhiteID,
			BlackId:      replay.BlackID,
			WhiteName:    replay.WhiteName,
			BlackName:    replay.BlackName,
			WhiteCountry: replay.WhiteCountry,
			BlackCountry: replay.BlackCountry,
			Mode:         replay.Mode,
			Result:       replay.Result,
			Cause:        replay.Cause,
			WinEloDiff:   replay.WinEloDiff,
			LoseEloDiff:  replay.LoseEloDiff,
			WhiteElo:     replay.WhiteElo,
			BlackElo:     replay.BlackElo,
			WhiteEloDiff: replay.WhiteEloDiff,
			BlackEloDiff: replay.BlackEloDiff,
			PlayedOn:     replay.PlayedOn.Format(time.RFC3339),
		}}},
	}
}
