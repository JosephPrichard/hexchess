package main

import "xk6-hexchess/pb"

func stringOfInput(input *pb.GameInput) string {
	switch input.GetValue().(type) {
	case *pb.GameInput_Forfeit:
		return "forfeit"
	case *pb.GameInput_Move:
		return "move"
	case *pb.GameInput_Chat:
		return "chat"
	case *pb.GameInput_Undo:
		return "undo"
	case *pb.GameInput_Ping:
		return "ping"
	default:
		return "unknown"
	}
}

func stringOfOutput(output *pb.GameOutput) string {
	switch output.GetValue().(type) {
	case *pb.GameOutput_Error:
		return "error"
	case *pb.GameOutput_Init:
		return "init"
	case *pb.GameOutput_Players:
		return "players"
	case *pb.GameOutput_Move:
		return "move"
	case *pb.GameOutput_Chat:
		return "chat"
	case *pb.GameOutput_Undo:
		return "undo"
	case *pb.GameOutput_Forfeit:
		return "forfeit"
	case *pb.GameOutput_Replay:
		return "replay"
	default:
		return "unknown"
	}
}
