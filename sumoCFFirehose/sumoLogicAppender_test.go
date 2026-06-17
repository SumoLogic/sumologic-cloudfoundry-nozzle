package sumoCFFirehose

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
	"time"

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
			"value":      float64(558108),
			"timestamp":  int64(1483629662001580569),
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
			"timestamp":  int64(1483629662001580569),
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
			"timestamp":  int64(1483629662001580569),
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

	// StringBuilder outputs Carbon2 text format for ValueMetric and CounterEvent
	// Timestamp (19-digit int64) is converted to seconds for metrics
	expectedTimestamp := int64(1483629662001580569) / int64(1000000000)
	expected := fmt.Sprintf("deployment=cf job_index=c82feee9-2159-4b05-b669-a9929eb59017 ip=10.193.166.33 job=cloud_controller origin=cc metric=requests.completed  unit=counter %f %d\n", float64(558108), expectedTimestamp) +
		fmt.Sprintf("deployment=cf-redis job_index=8081eca4-9e27-49cb-83ce-948e703c0939 ip=10.193.166.84 job=dedicated-node origin=MetronAgent metric=dropsondeMarshaller.sentEnvelopes_total  %d %d\n", 10249446, expectedTimestamp) +
		fmt.Sprintf("deployment=cf-redis job_index=8081eca4-9e27-49cb-83ce-948e703c0939 ip=10.193.166.84 job=dedicated-node origin=MetronAgent metric=dropsondeMarshaller.sentEnvelopes_delta  %d %d\n", 9, expectedTimestamp) +
		fmt.Sprintf("deployment=cf-redis job_index=23f9be01-bd83-4967-acba-69fc649f4ee6 ip=10.193.166.84 job=dedicated-node origin=MetronAgent metric=dropsondeAgentListener.receivedByteCount_total  %d %d\n", 639557085, expectedTimestamp) +
		fmt.Sprintf("deployment=cf-redis job_index=23f9be01-bd83-4967-acba-69fc649f4ee6 ip=10.193.166.84 job=dedicated-node origin=MetronAgent metric=dropsondeAgentListener.receivedByteCount_delta  %d %d\n", 582, expectedTimestamp)
	assert.Equal(t, expected, finalString, "")
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
	mapExpected := map[string][]string{
		"Key1": {"Value1"},
		"Key2": {"Value2"},
		"Key3": {"Value3"},
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
	includeOnlyFilter := "job:diego_cell,source_type:APP"

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
	includeOnlyFilter := "job:diego_cell,source_type:APP"
	excludeAlwaysFilter := "deployment:other,origin:router"
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
	includeOnlyFilter := "job:diego_cell,source_type:APP"
	excludeAlwaysFilter := "deployment:other,origin:reps"
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
	expectedTimestamp := time.Unix(0, int64(1483629662001580569)*int64(time.Nanosecond)).String()
	assert.Equal(t, expectedTimestamp, timestamp, "This timestamp should be in the string")
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
	// Non-19-digit int64 timestamps are left as-is by FormatTimestamp
	assert.Equal(t, int64(148362966200), timestamp, "Non-19-digit timestamp should be unchanged")
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
