package registry

type Notifications struct {
	Events []Event
}

const actionPush = "push"

type Event struct {
	Id        string
	Timestamp string
	Action    string
	Target    struct {
		Repository string
		Tag        string
	}
	// Request struct {}
	// Actor   struct {}
	// Source  struct {}
}

func (e Event) IsPush() bool {
	return e.Action == actionPush
}

func (e Event) HasTag() bool {
	return e.Target.Tag != ""
}
