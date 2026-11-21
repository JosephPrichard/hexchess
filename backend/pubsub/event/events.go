package event

type CountEventKind int

const (
	GlobalActiveEvent CountEventKind = iota
	GlobalGamesEvent
)

type CountEvent struct {
	Kind  CountEventKind `json:"kind"`
	Count int64          `json:"count"`
}
