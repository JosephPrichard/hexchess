package event

type CountEventKind int

const (
	ActiveCountEvent CountEventKind = iota
	GamesCountEvent
)

type CountEvent struct {
	Kind  CountEventKind `json:"kind"`
	Count int64          `json:"count"`
}
