package eventQueue

import (
	"testing"

	. "github.com/SumoLogic/sumologic-cloudfoundry-nozzle/events"
	"github.com/stretchr/testify/assert"
)

func TestQueueFIFO(t *testing.T) {
	assert := assert.New(t)
	event1 := Event{
		Fields: map[string]interface{}{
			"message_type": "OUT",
			"cf_app_id":    "011",
		},
		Msg: "index [01]",
	}

	event2 := Event{
		Fields: map[string]interface{}{
			"message_type": "OUT",
			"cf_app_id":    "022",
		},
		Msg: "index [02]",
	}

	event3 := Event{
		Fields: map[string]interface{}{
			"message_type": "OUT",
			"cf_app_id":    "033",
		},
		Msg: "index [03]",
	}

	queue := Queue{
		Events: make([]*Event, 3),
	}

	queue.Push(&event1)
	queue.Push(&event2)
	queue.Push(&event3)

	assert.Equal(queue.Pop().Msg, "index [01]", "")
	assert.Equal(queue.Pop().Msg, "index [02]", "")
	assert.Equal(queue.Pop().Msg, "index [03]", "")
}
