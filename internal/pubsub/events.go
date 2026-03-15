package pubsub

import "context"

const (
	CreatedEvent EventType = "created"
	UpdatedEvent EventType = "updated"
	DeletedEvent EventType = "deleted"
)

type (
	// EventType identifies the type of event
	EventType string

	// Event represents an event in the lifecycle of a resource
	Event[T any] struct {
		Type    EventType
		Payload T
	}

	Subscriber[T any] interface {
		Subscribe(context.Context) <-chan Event[T]
	}

	Publisher[T any] interface {
		Publish(EventType, T)
	}

	PubSub[T any] interface {
		Publisher[T]
		Subscriber[T]
	}
)
