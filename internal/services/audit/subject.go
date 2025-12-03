package audit

import "github.com/vshulcz/Golectra/pkg/observer"

// Observer receives audit events.
type Observer = observer.Observer[Event]

// ObserverFunc adapts a plain function to the Observer interface.
type ObserverFunc = observer.ObserverFunc[Event]

// Publisher broadcasts audit events.
type Publisher = observer.Publisher[Event]

// Subject fans out events to registered observers.
type Subject = observer.Subject[Event]

// NewSubject creates a subject optionally pre-populated with observers.
func NewSubject(observers ...Observer) *Subject {
	return observer.NewSubject[Event](observers...)
}
