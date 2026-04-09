package lsservice

// Event represents a lifecycle update emitted by the ls workflow.
type Event interface {
	isLSEvent()
}

// PreparedEvent reports that the workflow prerequisites were validated.
type PreparedEvent struct {
	Snapshot Snapshot
}

func (PreparedEvent) isLSEvent() {}

// ProgressEvent reports intermediate scan counters while ls is running.
type ProgressEvent struct {
	Progress Progress
}

func (ProgressEvent) isLSEvent() {}

// EventSink publishes service events to an external adapter.
type EventSink interface {
	Emit(Event)
}

// NoOpSink absorbs all events.
type NoOpSink struct{}

func (NoOpSink) Emit(Event) {}
