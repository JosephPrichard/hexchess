package svc

import (
	"encoding/json"
	"fmt"
	"google.golang.org/protobuf/proto"
	"hexchess-svc/chess"
	"hexchess-svc/pb"
	"time"
)

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

func SerializeEndState(es EndState) *pb.EndState {
	switch es.Kind {
	case Aborted:
		return &pb.EndState{Value: &pb.EndState_AbortState{}}
	case Finished:
		return &pb.EndState{Value: &pb.EndState_FinishState{
			FinishState: &pb.FinishState{
				WinEloDiff:  int32(es.WinEloDiff),
				LoseEloDiff: int32(es.LoseEloDiff),
				Cause:       es.Cause.String(),
				Result:      es.Result.String(),
			},
		}}
	default:
		return nil
	}
}

func DeserializeEndState(pbEndState *pb.EndState) (s EndState, err error) {
	if pbEndState == nil {
		return s, nil
	}
	switch p := pbEndState.GetValue().(type) {
	case *pb.EndState_FinishState:
		finishState := p.FinishState
		cause, err := ParseReplayCause(finishState.Cause)
		if err != nil {
			return s, err
		}
		result, err := ParseReplayResult(finishState.Result)
		if err != nil {
			return s, err
		}
		return EndState{
			Kind:        Finished,
			WinEloDiff:  int64(finishState.WinEloDiff),
			LoseEloDiff: int64(finishState.LoseEloDiff),
			Cause:       cause,
			Result:      result,
		}, nil
	case *pb.EndState_AbortState:
		return EndState{Kind: Aborted}, nil
	default:
		return s, fmt.Errorf("unknown end s type: %T", p)
	}
}

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
	color, err := ParseColor(pbChess.FirstColor)
	if err != nil {
		return st, err
	}
	mode, err := ParseGameMode(pbChess.Mode)
	if err != nil {
		return st, err
	}
	endState, err := DeserializeEndState(pbChess.EndState)
	if err != nil {
		return st, err
	}
	return ChessState{
		Game:         game,
		InitialBoard: initialBoard,
		UndoState:    UndoState{UndoID: pbChess.UndoId},
		EndState:     endState,
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

func SerializePlayer(p PlayerState) *pb.PlayerState {
	if p.Present {
		return &pb.PlayerState{Id: p.ID, Name: p.Name, Country: p.Country, IsGuest: p.IsGuest}
	}
	return nil
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
		EndState:     SerializeEndState(s.EndState),
	}
}

func UnmarshalChessMeta(b []byte) (m ChessMeta, err error) {
	var pbChess pb.ChessState
	if err := proto.Unmarshal(b, &pbChess); err != nil {
		return m, fmt.Errorf("unmarshal chess s: %w", err)
	}

	color, err := ParseColor(pbChess.FirstColor)
	if err != nil {
		return m, err
	}
	mode, err := ParseGameMode(pbChess.Mode)
	if err != nil {
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

func MarshalUserMsgJson(pbUm *pb.UserMsg) ([]byte, error) {
	if cm := pbUm.GetChallenge(); cm != nil {
		madeOn, err := time.Parse(time.RFC3339, cm.MadeOn)
		if err != nil {
			return nil, fmt.Errorf("parse challenge made on: %w", err)
		}
		return json.Marshal(ChallengeEntity{
			ChallengerID:      cm.ChallengerId,
			ChallengerName:    cm.ChallengerName,
			ChallengerCountry: cm.ChallengerCountry,
			ChallengerElo:     cm.ChallengerElo,
			ChallengeeID:      cm.ChallengeeId,
			ChallengeeName:    cm.ChallengeeName,
			ChallengeeCountry: cm.ChallengeeCountry,
			ChallengeeElo:     cm.ChallengeeElo,
			StartColor:        cm.StartColor,
			Mode:              cm.Mode,
			MadeOn:            madeOn,
		})
	}
	return nil, fmt.Errorf("unknown message type: %T", pbUm)
}

func SerializeChallengeMsg(ce ChallengeEntity) *pb.UserMsg {
	cm := &pb.UserMsg_Challenge{
		Challenge: &pb.ChallengeMsg{
			ChallengerId:      ce.ChallengerID,
			ChallengerName:    ce.ChallengerName,
			ChallengerCountry: ce.ChallengerCountry,
			ChallengerElo:     ce.ChallengerElo,
			ChallengeeId:      ce.ChallengeeID,
			ChallengeeName:    ce.ChallengeeName,
			ChallengeeCountry: ce.ChallengeeCountry,
			ChallengeeElo:     ce.ChallengeeElo,
			Mode:              ce.Mode,
			StartColor:        ce.StartColor,
			MadeOn:            ce.MadeOn.Format(time.RFC3339),
		},
	}
	return &pb.UserMsg{UserId: ce.ChallengeeID, Value: cm}
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
