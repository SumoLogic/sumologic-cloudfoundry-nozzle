package sumoCFFirehose

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/SumoLogic/sumologic-cloudfoundry-nozzle/eventQueue"
	. "github.com/SumoLogic/sumologic-cloudfoundry-nozzle/events"
)

func TestAppenderStringBuilder(t *testing.T) {
	event1 := Event{
		Fields: map[string]interface{}{
			"deployment": "cf",
			"ip":         "10.193.166.33",
			"job":        "cloud_controller",
			"job_index":  "c82feee9-2159-4b05-b669-a9929eb59017",
			"name":       "requests.completed",
			"origin":     "cc",
			"unit":       "counter",
			"value":      558108,
		},
		Msg:  "",
		Type: "ValueMetric",
	}

	event2 := Event{
		Fields: map[string]interface{}{
			"delta":      9,
			"deployment": "cf-redis",
			"ip":         "10.193.166.84",
			"job":        "dedicated-node",
			"job_index":  "8081eca4-9e27-49cb-83ce-948e703c0939",
			"name":       "dropsondeMarshaller.sentEnvelopes",
			"origin":     "MetronAgent",
			"total":      10249446,
		},
		Msg:  "",
		Type: "CounterEvent",
	}

	event3 := Event{
		Fields: map[string]interface{}{
			"delta":      582,
			"deployment": "cf-redis",
			"ip":         "10.193.166.84",
			"job":        "dedicated-node",
			"job_index":  "23f9be01-bd83-4967-acba-69fc649f4ee6",
			"name":       "dropsondeAgentListener.receivedByteCount",
			"origin":     "MetronAgent",
			"total":      639557085,
		},
		Msg:  "",
		Type: "CounterEvent",
	}
	queue := Queue{
		Events: make([]*Event, 3),
	}
	queue.Push(&event1)
	queue.Push(&event2)
	queue.Push(&event3)

	finalString := ""
	for queue.GetCount() > 0 {
		finalString = finalString + StringBuilder(queue.Pop(), true, "", "", "")
	}
	assert.Equal(t, finalString, "{\"Fields\":{\"deployment\":\"cf\",\"ip\":\"10.193.166.33\",\"job\":\"cloud_controller\",\"job_index\":\"c82feee9-2159-4b05-b669-a9929eb59017\",\"name\":\"requests.completed\",\"origin\":\"cc\",\"unit\":\"counter\",\"value\":558108},\"Msg\":\"\",\"Type\":\"ValueMetric\"}\n"+
		"{\"Fields\":{\"delta\":9,\"deployment\":\"cf-redis\",\"ip\":\"10.193.166.84\",\"job\":\"dedicated-node\",\"job_index\":\"8081eca4-9e27-49cb-83ce-948e703c0939\",\"name\":\"dropsondeMarshaller.sentEnvelopes\",\"origin\":\"MetronAgent\",\"total\":10249446},\"Msg\":\"\",\"Type\":\"CounterEvent\"}\n"+
		"{\"Fields\":{\"delta\":582,\"deployment\":\"cf-redis\",\"ip\":\"10.193.166.84\",\"job\":\"dedicated-node\",\"job_index\":\"23f9be01-bd83-4967-acba-69fc649f4ee6\",\"name\":\"dropsondeAgentListener.receivedByteCount\",\"origin\":\"MetronAgent\",\"total\":639557085},\"Msg\":\"\",\"Type\":\"CounterEvent\"}\n", "")
}

func TestStringBuilderVerboseLogsFalse(t *testing.T) {
	eventVerboseLogMessage := Event{
		Fields: map[string]interface{}{
			"message_type":    "OUT",
			"source_instance": 0,
			"deployment":      "cf",
			"ip":              "10.193.166.47",
			"job":             "diego_cell",
			"job_index":       "c62aebe5-16b8-43f5-a589-1267e09b9537",
			"cf_ignored_app":  "false",
			"timestamp":       int64(1483629662001580713),
			"source_type":     "APP",
			"origin":          "rep",
			"cf_app_id":       "7833dc75-4484-409c-9b74-90b6454906c6",
		},
		Msg:  "Triggering 'app usage events fetcher'",
		Type: "LogMessage",
	}

	finalMessage := StringBuilder(&eventVerboseLogMessage, false, "", "", "")
	assert.NotContains(t, finalMessage, "source_type", "dsds")

}

func TestStringBuilderVerboseLogsTrue(t *testing.T) {
	eventVerboseLogMessage := Event{
		Fields: map[string]interface{}{
			"message_type":    "OUT",
			"source_instance": 0,
			"deployment":      "cf",
			"ip":              "10.193.166.47",
			"job":             "diego_cell",
			"job_index":       "c62aebe5-16b8-43f5-a589-1267e09b9537",
			"cf_ignored_app":  "false",
			"timestamp":       int64(1483629662001580713),
			"source_type":     "APP",
			"origin":          "rep",
			"cf_app_id":       "7833dc75-4484-409c-9b74-90b6454906c6",
		},
		Msg:  "Triggering 'app usage events fetcher'",
		Type: "LogMessage",
	}
	finalMessage := StringBuilder(&eventVerboseLogMessage, true, "", "", "")

	assert.Contains(t, finalMessage, "source_type", "")

}
func TestSendParseCustomMetadata(t *testing.T) {
	customMetadata := "Key1:Value1,Key2:Value2,Key3:Value3"
	mapCustomMetadata := ParseCustomInput(customMetadata)
	mapExpected := map[string]string{
		"Key1": "Value1",
		"Key2": "Value2",
		"Key3": "Value3",
	}

	assert.Equal(t, mapExpected, mapCustomMetadata, "")

}
func TestSendIncludeOnlyFilter(t *testing.T) {
	eventToInclude := Event{
		Fields: map[string]interface{}{
			"message_type":    "OUT",
			"source_instance": 0,
			"deployment":      "cf",
			"ip":              "10.193.166.47",
			"job":             "diego_cell",
			"job_index":       "c62aebe5-16b8-43f5-a589-1267e09b9537",
			"cf_ignored_app":  "false",
			"timestamp":       "2017-01-10 17:31:02.662133274 -0300 CLST",
			"source_type":     "APP",
			"origin":          "rep",
			"cf_app_id":       "7833dc75-4484-409c-9b74-90b6454906c6",
		},
		Msg:  "Triggering 'app usage events fetcher'",
		Type: "LogMessage",
	}
	message, err := json.Marshal(eventToInclude)
	var msg []byte
	if err == nil {
		msg = message
	}
	buf := new(bytes.Buffer)
	buf.Write(msg)
	includeOnlyFilter := "job:diego_cell,source_type:other"

	assert.True(t, WantedEvent(buf.String(), includeOnlyFilter, ""), "This Event should be included")
}
func TestSendExcludeAlwaysFilter(t *testing.T) {
	eventToExclude := Event{
		Fields: map[string]interface{}{
			"message_type":    "OUT",
			"source_instance": 0,
			"deployment":      "cf",
			"ip":              "10.193.166.47",
			"job":             "diego_cell",
			"job_index":       "c62aebe5-16b8-43f5-a589-1267e09b9537",
			"cf_ignored_app":  "false",
			"timestamp":       "2017-01-10 17:31:02.662133274 -0300 CLST",
			"source_type":     "APP",
			"origin":          "rep",
			"cf_app_id":       "7833dc75-4484-409c-9b74-90b6454906c6",
		},
		Msg:  "Triggering 'app usage events fetcher'",
		Type: "LogMessage",
	}
	message, err := json.Marshal(eventToExclude)
	var msg []byte
	if err == nil {
		msg = message
	}
	buf := new(bytes.Buffer)
	buf.Write(msg)
	excludeAlwaysFilter := "source_type:other,cf_app_id:7833dc75-4484-409c-9b74-90b6454906c6"
	assert.False(t, WantedEvent(buf.String(), "", excludeAlwaysFilter), "This Event should be excluded")
}
func TestSendIncludeExcludeFilterOverride(t *testing.T) {
	eventToExclude := Event{
		Fields: map[string]interface{}{
			"message_type":    "OUT",
			"source_instance": 0,
			"deployment":      "cf",
			"ip":              "10.193.166.47",
			"job":             "diego_cell",
			"job_index":       "c62aebe5-16b8-43f5-a589-1267e09b9537",
			"cf_ignored_app":  "false",
			"timestamp":       "2017-01-10 17:31:02.662133274 -0300 CLST",
			"source_type":     "APP",
			"origin":          "rep",
			"cf_app_id":       "7833dc75-4484-409c-9b74-90b6454906c6",
		},
		Msg:  "Triggering 'app usage events fetcher'",
		Type: "LogMessage",
	}
	message, err := json.Marshal(eventToExclude)
	var msg []byte
	if err == nil {
		msg = message
	}
	buf := new(bytes.Buffer)
	buf.Write(msg)
	includeOnlyFilter := "job:diego_cell,source_type:other"
	excludeAlwaysFilter := "source_type:other,cf_app_id:7833dc75-4484-409c-9b74-90b6454906c6"
	assert.False(t, WantedEvent(buf.String(), includeOnlyFilter, excludeAlwaysFilter), "This Event should be not included, override filter")
}

func TestSendIncludeExcludeFilterAppMatchIncluded(t *testing.T) {
	eventToExclude := Event{
		Fields: map[string]interface{}{
			"message_type":    "OUT",
			"source_instance": 0,
			"deployment":      "cf",
			"ip":              "10.193.166.47",
			"job":             "diego_cell",
			"job_index":       "c62aebe5-16b8-43f5-a589-1267e09b9537",
			"cf_ignored_app":  "false",
			"timestamp":       "2017-01-10 17:31:02.662133274 -0300 CLST",
			"source_type":     "APP",
			"origin":          "rep",
			"cf_app_id":       "7833dc75-4484-409c-9b74-90b6454906c6",
		},
		Msg:  "Triggering 'app usage events fetcher'",
		Type: "LogMessage",
	}
	message, err := json.Marshal(eventToExclude)
	var msg []byte
	if err == nil {
		msg = message
	}
	buf := new(bytes.Buffer)
	buf.Write(msg)
	includeOnlyFilter := "job:diego_cell,source_type:other"
	excludeAlwaysFilter := "source_type:other,origin:router"
	assert.True(t, WantedEvent(buf.String(), includeOnlyFilter, excludeAlwaysFilter), "This Event should be included")
}

func TestSendIncludeExcludeFilterAppMatchExcluded(t *testing.T) {
	eventToExclude := Event{
		Fields: map[string]interface{}{
			"message_type":    "OUT",
			"source_instance": 0,
			"deployment":      "cf",
			"ip":              "10.193.166.47",
			"job":             "diego_cell",
			"job_index":       "c62aebe5-16b8-43f5-a589-1267e09b9537",
			"cf_ignored_app":  "false",
			"timestamp":       "2017-01-10 17:31:02.662133274 -0300 CLST",
			"source_type":     "APP",
			"origin":          "rep",
			"cf_app_id":       "7833dc75-4484-409c-9b74-90b6454906c6",
		},
		Msg:  "Triggering 'app usage events fetcher'",
		Type: "LogMessage",
	}
	message, err := json.Marshal(eventToExclude)
	var msg []byte
	if err == nil {
		msg = message
	}
	buf := new(bytes.Buffer)
	buf.Write(msg)
	includeOnlyFilter := "job:dedicated-node,source_type:other"
	excludeAlwaysFilter := "source_type:other,origin:rep"
	assert.False(t, WantedEvent(buf.String(), includeOnlyFilter, excludeAlwaysFilter), "This Event should not be included")
}

func TestSendIncludeExcludeFilterAppNotMatchAnyFilter(t *testing.T) {
	eventToExclude := Event{
		Fields: map[string]interface{}{
			"message_type":    "OUT",
			"source_instance": 0,
			"deployment":      "cf",
			"ip":              "10.193.166.47",
			"job":             "diego_cell",
			"job_index":       "c62aebe5-16b8-43f5-a589-1267e09b9537",
			"cf_ignored_app":  "false",
			"timestamp":       "2017-01-10 17:31:02.662133274 -0300 CLST",
			"source_type":     "APP",
			"origin":          "rep",
			"cf_app_id":       "7833dc75-4484-409c-9b74-90b6454906c6",
		},
		Msg:  "Triggering 'app usage events fetcher'",
		Type: "LogMessage",
	}
	message, err := json.Marshal(eventToExclude)
	var msg []byte
	if err == nil {
		msg = message
	}
	buf := new(bytes.Buffer)
	buf.Write(msg)
	includeOnlyFilter := "job:dedicated-node,source_type:other"
	excludeAlwaysFilter := "source_type:other,origin:reps"
	assert.True(t, WantedEvent(buf.String(), includeOnlyFilter, excludeAlwaysFilter), "This Event should be included")
}

func TestSendNoFilter(t *testing.T) {
	eventToExclude := Event{
		Fields: map[string]interface{}{
			"message_type":    "OUT",
			"source_instance": 0,
			"deployment":      "cf",
			"ip":              "10.193.166.47",
			"job":             "diego_cell",
			"job_index":       "c62aebe5-16b8-43f5-a589-1267e09b9537",
			"cf_ignored_app":  "false",
			"timestamp":       "2017-01-10 17:31:02.662133274 -0300 CLST",
			"source_type":     "APP",
			"origin":          "rep",
			"cf_app_id":       "7833dc75-4484-409c-9b74-90b6454906c6",
		},
		Msg:  "Triggering 'app usage events fetcher'",
		Type: "LogMessage",
	}
	message, err := json.Marshal(eventToExclude)
	var msg []byte
	if err == nil {
		msg = message
	}
	buf := new(bytes.Buffer)
	buf.Write(msg)
	assert.True(t, WantedEvent(buf.String(), "", ""), "This Event should be included")
}

func TestSendStringTimestamp(t *testing.T) {
	eventStringTimestamp := Event{
		Fields: map[string]interface{}{
			"message_type":    "OUT",
			"source_instance": 0,
			"deployment":      "cf",
			"ip":              "10.193.166.47",
			"job":             "diego_cell",
			"job_index":       "c62aebe5-16b8-43f5-a589-1267e09b9537",
			"cf_ignored_app":  "false",
			"timestamp":       "1483629662001580569",
			"source_type":     "APP",
			"origin":          "rep",
			"cf_app_id":       "7833dc75-4484-409c-9b74-90b6454906c6",
		},
		Msg:  "Triggering 'app usage events fetcher'",
		Type: "LogMessage",
	}
	FormatTimestamp(&eventStringTimestamp, "timestamp")
	timestamp := eventStringTimestamp.Fields["timestamp"]
	assert.Equal(t, timestamp, "1483629662001580569", "This timestamp should be in the string")
}

func TestSendInt64Timestamp19(t *testing.T) {
	eventStringTimestamp := Event{
		Fields: map[string]interface{}{
			"message_type":    "OUT",
			"source_instance": 0,
			"deployment":      "cf",
			"ip":              "10.193.166.47",
			"job":             "diego_cell",
			"job_index":       "c62aebe5-16b8-43f5-a589-1267e09b9537",
			"cf_ignored_app":  "false",
			"timestamp":       int64(1483629662001580569),
			"source_type":     "APP",
			"origin":          "rep",
			"cf_app_id":       "7833dc75-4484-409c-9b74-90b6454906c6",
		},
		Msg:  "Triggering 'app usage events fetcher'",
		Type: "LogMessage",
	}
	FormatTimestamp(&eventStringTimestamp, "timestamp")
	timestamp := eventStringTimestamp.Fields["timestamp"]
	assert.Equal(t, timestamp, "2017-01-05 12:21:02.001580569 -0300 CLST", "This timestamp should be in the string")
}
func TestSendInt64Timestamp14(t *testing.T) {
	eventStringTimestamp := Event{
		Fields: map[string]interface{}{
			"message_type":    "OUT",
			"source_instance": 0,
			"deployment":      "cf",
			"ip":              "10.193.166.47",
			"job":             "diego_cell",
			"job_index":       "c62aebe5-16b8-43f5-a589-1267e09b9537",
			"cf_ignored_app":  "false",
			"timestamp":       int64(148362966200),
			"source_type":     "APP",
			"origin":          "rep",
			"cf_app_id":       "7833dc75-4484-409c-9b74-90b6454906c6",
		},
		Msg:  "Triggering 'app usage events fetcher'",
		Type: "LogMessage",
	}
	FormatTimestamp(&eventStringTimestamp, "timestamp")
	timestamp := eventStringTimestamp.Fields["timestamp"]
	assert.Equal(t, timestamp, "", "This timestamp should be in the string")
}
func TestSendWrongTimestampField(t *testing.T) {
	eventStringTimestamp := Event{
		Fields: map[string]interface{}{
			"message_type":    "OUT",
			"source_instance": 0,
			"deployment":      "cf",
			"ip":              "10.193.166.47",
			"job":             "diego_cell",
			"job_index":       "c62aebe5-16b8-43f5-a589-1267e09b9537",
			"cf_ignored_app":  "false",
			"timestamp":       int64(148362966200),
			"source_type":     "APP",
			"origin":          "rep",
			"cf_app_id":       "7833dc75-4484-409c-9b74-90b6454906c6",
		},
		Msg:  "Triggering 'app usage events fetcher'",
		Type: "LogMessage",
	}
	assert.NotPanics(t, func() { FormatTimestamp(&eventStringTimestamp, "timestamp2") }, "msgAndArgs")
}
func TestSendNotIntNotStringTimestampField(t *testing.T) {
	eventStringTimestamp := Event{
		Fields: map[string]interface{}{
			"message_type":    "OUT",
			"source_instance": 0,
			"deployment":      "cf",
			"ip":              "10.193.166.47",
			"job":             "diego_cell",
			"job_index":       "c62aebe5-16b8-43f5-a589-1267e09b9537",
			"cf_ignored_app":  "false",
			"timestamp":       float32(148362966200),
			"source_type":     "APP",
			"origin":          "rep",
			"cf_app_id":       "7833dc75-4484-409c-9b74-90b6454906c6",
		},
		Msg:  "Triggering 'app usage events fetcher'",
		Type: "LogMessage",
	}
	FormatTimestamp(&eventStringTimestamp, "timestamp")
	timestamp := eventStringTimestamp.Fields["timestamp"]
	assert.Equal(t, timestamp, "", "This timestamp should be in the string")
}
