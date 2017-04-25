package eventQueue

import . "github.com/SumoLogic/sumologic-cloudfoundry-nozzle/events"

// Queue is a basic FIFO queue based on a circular list that resizes as needed.
type Queue struct {
	Events []*Event
	head   int
	tail   int
	count  int
}

func NewQueue(n []*Event) Queue {
	return Queue{
		Events: n,
	}
}

func (q *Queue) GetNode() []*Event {
	return q.Events
}

func (q *Queue) GetCount() int {
	return q.count
}

/*
func (q *Queue) GetEvents() Event {
	return q.Events
}*/

// Push adds a node to the queue.
func (q *Queue) Push(n *Event) {
	if q.head == q.tail && q.count > 0 {
		events := make([]*Event, len(q.Events)*2)
		copy(events, q.Events[q.head:])
		copy(events[len(q.Events)-q.head:], q.Events[:q.head])
		q.head = 0
		q.tail = len(q.Events)
		q.Events = events
	}
	q.Events[q.tail] = n
	q.tail = (q.tail + 1) % len(q.Events)
	q.count++
}

// Pop removes and returns a node from the queue in first to last order.
func (q *Queue) Pop() *Event {
	if q.count == 0 {
		return nil
	}
	node := q.Events[q.head]
	q.head = (q.head + 1) % len(q.Events)
	q.count--
	return node
}
