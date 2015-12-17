package airbrake

type Event struct {
	Type string

	Context    map[string]interface{}
	Stacktrace *Stacktrace
}

func NewEvent(t string) *Event {
	e := Event{
		Type:    t,
		Context: make(map[string]interface{}),
	}
	return &e
}
