package eventRouting

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/SumoLogic/sumologic-cloudfoundry-nozzle/caching"
	"github.com/SumoLogic/sumologic-cloudfoundry-nozzle/eventQueue"
	fevents "github.com/SumoLogic/sumologic-cloudfoundry-nozzle/events"
	"github.com/cloudfoundry/sonde-go/events"
)

type EventRouting struct {
	CachingClient       caching.Caching
	selectedEvents      map[string]bool
	selectedEventsCount map[string]uint64
	mutex               *sync.Mutex
	queue               *eventQueue.Queue
}

func NewEventRouting(caching caching.Caching, queue *eventQueue.Queue) *EventRouting {
	return &EventRouting{
		CachingClient:       caching,
		selectedEvents:      make(map[string]bool),
		selectedEventsCount: make(map[string]uint64),
		queue:               queue,
		mutex:               &sync.Mutex{},
	}
}

func (e *EventRouting) GetSelectedEvents() map[string]bool {
	return e.selectedEvents
}

func (e *EventRouting) RouteEvent(msg *events.Envelope) {

	eventType := msg.GetEventType()

	if e.selectedEvents[eventType.String()] {
		var event *fevents.Event
		switch eventType {
		case events.Envelope_HttpStart:
			event = fevents.HttpStart(msg)
		case events.Envelope_HttpStop:
			event = fevents.HttpStop(msg)
		case events.Envelope_HttpStartStop:
			event = fevents.HttpStartStop(msg)
		case events.Envelope_LogMessage:
			event = fevents.LogMessage(msg)
		case events.Envelope_ValueMetric:
			event = fevents.ValueMetric(msg)
		case events.Envelope_CounterEvent:
			event = fevents.CounterEvent(msg)
		case events.Envelope_Error:
			event = fevents.ErrorEvent(msg)
		case events.Envelope_ContainerMetric:
			event = fevents.ContainerMetric(msg)
		}

		event.AnnotateWithEnveloppeData(msg)

		if _, hasAppId := event.Fields["cf_app_id"]; hasAppId {
			event.AnnotateWithAppData(e.CachingClient)
		}

		e.mutex.Lock()
		//We do not ship Event
		if ignored, hasIgnoredField := event.Fields["cf_ignored_app"]; ignored == true && hasIgnoredField {
			e.selectedEventsCount["ignored_app_message"]++
		} else {
			//Push the event to the queue
			e.queue.Push(event)
			e.selectedEventsCount[eventType.String()]++
		}
		e.mutex.Unlock()
	}
}

func (e *EventRouting) SetupEventRouting(wantedEvents string) error {
	e.selectedEvents = make(map[string]bool)
	if wantedEvents == "" {
		e.selectedEvents["LogMessage"] = true
	} else {
		for _, event := range strings.Split(wantedEvents, ",") {
			if e.isAuthorizedEvent(strings.TrimSpace(event)) {
				e.selectedEvents[strings.TrimSpace(event)] = true
				//logging.LogStd(fmt.Sprintf("Event Type [%s] is included in the fireshose!", event), false)
			} else {
				return fmt.Errorf("Rejected Event Name [%s] - Valid events: %s", event, GetListAuthorizedEventEvents())
			}
		}
	}
	return nil
}

func (e *EventRouting) isAuthorizedEvent(wantedEvent string) bool {
	for _, authorizeEvent := range events.Envelope_EventType_name {
		if wantedEvent == authorizeEvent {
			return true
		}
	}
	return false
}

func GetListAuthorizedEventEvents() (authorizedEvents string) {
	arrEvents := []string{}
	for _, listEvent := range events.Envelope_EventType_name {
		arrEvents = append(arrEvents, listEvent)
	}
	sort.Strings(arrEvents)
	return strings.Join(arrEvents, ", ")
}

/*func (e *EventRouting) GetTotalCountOfSelectedEvents() uint64 {
	var total = uint64(0)
	for _, count := range e.GetSelectedEventsCount() {
		total += count
	}
	return total
}

func (e *EventRouting) GetSelectedEventsCount() map[string]uint64 {
	return e.selectedEventsCount
}

func (e *EventRouting) LogEventTotals(logTotalsTime time.Duration) {
	firehoseEventTotals := time.NewTicker(logTotalsTime)
	count := uint64(0)
	startTime := time.Now()
	totalTime := startTime

	go func() {
		for range firehoseEventTotals.C {
			elapsedTime := time.Since(startTime).Seconds()
			totalElapsedTime := time.Since(totalTime).Seconds()
			startTime = time.Now()
			event, lastCount := e.getEventTotals(totalElapsedTime, elapsedTime, count)
			count = lastCount

			//Push the event to the queue
			e.queue.Push(event)
		}
	}()
}*/

/*func (e *EventRouting) getEventTotals(totalElapsedTime float64, elapsedTime float64, lastCount uint64) (*fevents.Event, uint64) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	totalCount := e.GetTotalCountOfSelectedEvents()
	sinceLastTime := float64(int(elapsedTime*10)) / 10
	fields := logrus.Fields{
		"total_count":   totalCount,
		"by_sec_Events": int((totalCount - lastCount) / uint64(sinceLastTime)),
	}

	for eventtype, count := range e.GetSelectedEventsCount() {
		fields[eventtype] = count
	}

	event := &fevents.Event{
		Type:   "firehose_to_syslog_stats",
		Msg:    "Statistic for firehose to syslog",
		Fields: fields,
	}
	event.AnnotateWithMetaData(map[string]string{})
	return event, totalCount
}
*/
