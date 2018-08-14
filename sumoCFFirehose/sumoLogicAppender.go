package sumoCFFirehose

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/SumoLogic/sumologic-cloudfoundry-nozzle/eventQueue"
	"github.com/SumoLogic/sumologic-cloudfoundry-nozzle/events"
	"github.com/SumoLogic/sumologic-cloudfoundry-nozzle/logging"
)

type SumoLogicAppender struct {
	url                         string
	connectionTimeout           int //10000
	httpClient                  http.Client
	nozzleQueue                 *eventQueue.Queue
	eventsBatchSize             int
	sumoPostMinimumDelay        time.Duration
	timerBetweenPost            time.Time
	sumoCategory                string
	sumoName                    string
	sumoHost                    string
	verboseLogMessages          bool
	customMetadata              string
	includeOnlyMatchingFilter   string
	excludeAlwaysMatchingFilter string
	nozzleVersion               string
	logDelay					time.Time
}

type SumoBuffer struct {
	eventsInCurrentBuffer 	int
	logStringToSend   		*bytes.Buffer
	metricStringToSend		*bytes.Buffer
	timerIdlebuffer         time.Time
}

func NewSumoLogicAppender(urlValue string, connectionTimeoutValue int, nozzleQueue *eventQueue.Queue, eventsBatchSize int, sumoPostMinimumDelay time.Duration, sumoCategory string, sumoName string, sumoHost string, verboseLogMessages bool, customMetadata string, includeOnlyMatchingFilter string, excludeAlwaysMatchingFilter string, nozzleVersion string) *SumoLogicAppender {
	return &SumoLogicAppender{
		url:                         urlValue,
		connectionTimeout:           connectionTimeoutValue,
		httpClient:                  http.Client{Timeout: time.Duration(connectionTimeoutValue * int(time.Millisecond))},
		nozzleQueue:                 nozzleQueue,
		eventsBatchSize:             eventsBatchSize,
		sumoPostMinimumDelay:        sumoPostMinimumDelay,
		sumoCategory:                sumoCategory,
		sumoName:                    sumoName,
		sumoHost:                    sumoHost,
		verboseLogMessages:          verboseLogMessages,
		customMetadata:              customMetadata,
		includeOnlyMatchingFilter:   includeOnlyMatchingFilter,
		excludeAlwaysMatchingFilter: excludeAlwaysMatchingFilter,
		nozzleVersion:               nozzleVersion,
	}
}

func newBuffer() SumoBuffer {
	return SumoBuffer{
		eventsInCurrentBuffer:	0,
		logStringToSend:	    bytes.NewBufferString(""),
		metricStringToSend:     bytes.NewBufferString(""),
	}
}

func (s *SumoLogicAppender) Start() {
	s.timerBetweenPost = time.Now()
	Buffer := newBuffer()
	Buffer.timerIdlebuffer = time.Now()
	s.logDelay = time.Now()
	logging.Info.Println("Starting Appender Worker")
	for {
		if time.Since(s.logDelay).Seconds() >= 10 {
			logging.Info.Printf("Log queue size: %d", s.nozzleQueue.GetCount())
			s.logDelay = time.Now()
		}

		if s.nozzleQueue.GetCount() == 0 {
			logging.Trace.Println("Waiting for 300 ms")
			time.Sleep(300 * time.Millisecond)
		}

		if time.Since(Buffer.timerIdlebuffer).Seconds() >= 10 && Buffer.eventsInCurrentBuffer > 0 {
			logging.Info.Println("Sending batch after timer exceeded... #of Events: ", Buffer.eventsInCurrentBuffer)

			go s.SendToSumo(Buffer.logStringToSend.String(), s.url, false)
			go s.SendToSumo(Buffer.metricStringToSend.String(), s.url, true)

			Buffer = newBuffer()
			Buffer.timerIdlebuffer = time.Now()
			continue
		}

		if s.nozzleQueue.GetCount() != 0 {
			queueCount := s.nozzleQueue.GetCount()
			remainingBufferCount := s.eventsBatchSize - Buffer.eventsInCurrentBuffer
			if queueCount >= remainingBufferCount {
				logging.Trace.Println("Pushing Logs to Sumo: ")
				logging.Trace.Println(remainingBufferCount)
				for i := 0; i < remainingBufferCount; i++ {
					s.AppendLogs(&Buffer)
					Buffer.timerIdlebuffer = time.Now()
				}

				go s.SendToSumo(Buffer.logStringToSend.String(), s.url, false)
				go s.SendToSumo(Buffer.metricStringToSend.String(), s.url, true)

				Buffer = newBuffer()
			} else {
				logging.Trace.Println("Pushing Logs to Buffer: ")
				logging.Trace.Println(queueCount)
				for i := 0; i < queueCount; i++ {
					s.AppendLogs(&Buffer)
					Buffer.timerIdlebuffer = time.Now()
				}
			}
		}
	}
}

func WantedEvent(event string, includeOnlyMatchingFilter string, excludeAlwaysMatchingFilter string) bool {
	if includeOnlyMatchingFilter != "" {
		subslice := ParseCustomInput(includeOnlyMatchingFilter)
		for key, values := range subslice {
			if (strings.Contains(event, "\""+key+"\":\"") || strings.Contains(event, key+"=")) {
				include := false
				for _, value := range values {
					if strings.Contains(event, "\""+key+"\":\""+value+"\"") || strings.Contains(event, key+"="+value+" ") {
						include = true
						break
					}
				}
				if !include {
					return false
				}
			}
		}
	}
	if excludeAlwaysMatchingFilter != "" {
		subslice := ParseCustomInput(excludeAlwaysMatchingFilter)
		for key, values := range subslice {
			if (strings.Contains(event, "\""+key+"\":\"") || strings.Contains(event, key+"=")) {
				for _, value := range values {
					if strings.Contains(event, "\""+key+"\":\""+value+"\"") || strings.Contains(event, key+"="+value+" ") {
						return false
					}
				}
			}
		}
	}
	return true
}

func FormatTimestamp(event *events.Event, timestamp string) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in f", r)
		}
	}()

	if reflect.TypeOf(event.Fields[timestamp]).Kind() == reflect.String {
		event.Fields[timestamp] = event.Fields[timestamp].(string)
	} else if reflect.TypeOf(event.Fields[timestamp]).Kind() == reflect.Int64 {
		if len(strconv.FormatInt(event.Fields[timestamp].(int64), 10)) == 19 {		
			if event.Type == "ValueMetric" || event.Type == "CounterEvent" || event.Type == "ContainerMetric" {
				event.Fields[timestamp] = event.Fields[timestamp].(int64) / int64(time.Second)
			} else {
				event.Fields[timestamp] = time.Unix(0, event.Fields[timestamp].(int64)*int64(time.Nanosecond)).String()
			}
		} // if len(strconv.FormatInt(event.Fields[timestamp].(int64), 10)) == 19 -> leave it as is
	} else {
		event.Fields[timestamp] = ""
	}
}

func StringBuilder(event *events.Event, verboseLogMessages bool, includeOnlyMatchingFilter string, excludeAlwaysMatchingFilter string, customMetadata string) string {
	if customMetadata != "" {
		customMetadataMap := ParseCustomInput(customMetadata)
		for key, value := range customMetadataMap {
			event.Fields[key] = value
		}
	}
	eventType := event.Type
	var msg []byte
	switch eventType {
	case "HttpStart":
		FormatTimestamp(event, "timestamp")
		message, err := json.Marshal(event)
		if err == nil {
			msg = message
		}
	case "HttpStop":
		FormatTimestamp(event, "timestamp")
		message, err := json.Marshal(event)
		if err == nil {
			msg = message
		}
	case "HttpStartStop":
		FormatTimestamp(event, "start_timestamp")
		FormatTimestamp(event, "stop_timestamp")
		message, err := json.Marshal(event)
		if err == nil {
			msg = message
		}
	case "LogMessage":
		FormatTimestamp(event, "timestamp")
		if verboseLogMessages == true {
			message, err := json.Marshal(event)
			if err == nil {
				msg = message
			}
		} else {
			eventNoVerbose := events.Event{
				Fields: map[string]interface{}{
					"timestamp":   event.Fields["timestamp"],
					"cf_app_id": event.Fields["cf_app_id"],
					"deployment": event.Fields["deployment"],
					"job_index": event.Fields["job_index"],
					"job": event.Fields["job"],
					"ip": event.Fields["ip"],
					"origin": event.Fields["origin"],
				},
				Msg:  event.Msg,
				Type: event.Type,
			}
			if customMetadata != "" {
				customMetadataMap := ParseCustomInput(customMetadata)
				for key, value := range customMetadataMap {
					eventNoVerbose.Fields[key] = value
				}
			}
			message, err := json.Marshal(eventNoVerbose)
			if err == nil {
				msg = message
			}
		}
	case "ValueMetric":
		FormatTimestamp(event, "timestamp")
		deployment := event.Fields["deployment"]
		jobIndex := event.Fields["job_index"]
		ipString := event.Fields["ip"]
		ip := ""
		if ipString != nil && ipString != "" {
			ip = fmt.Sprintf(" ip=%s", ipString)
		}
		job := event.Fields["job"]
		origin := event.Fields["origin"]
		eventUnit := event.Fields["unit"]
		units := ""
		if eventUnit != nil && eventUnit != "" {
			units = fmt.Sprintf(" unit=%s", eventUnit)
		}
		msg = []byte(fmt.Sprintf("deployment=%s job_index=%s%s job=%s origin=%s metric=%s %s %f %d", 
			deployment, jobIndex, ip, job, origin, event.Fields["name"], units, event.Fields["value"], event.Fields["timestamp"]))
	case "CounterEvent":
		FormatTimestamp(event, "timestamp")
		deployment := event.Fields["deployment"]
		jobIndex := event.Fields["job_index"]
		ipString := event.Fields["ip"]
		ip := ""
		if ipString != nil && ipString != "" {
			ip = fmt.Sprintf(" ip=%s", ipString)
		}
		job := event.Fields["job"]
		origin := event.Fields["origin"]
		name := event.Fields["name"]
		timestamp := event.Fields["timestamp"]
		msg = []byte(fmt.Sprintf("deployment=%s job_index=%s%s job=%s origin=%s metric=%s_total  %d %d\n" +
			"deployment=%s job_index=%s%s job=%s origin=%s metric=%s_delta  %d %d", 
			deployment, jobIndex, ip, job, origin, name, event.Fields["total"], timestamp,
			deployment, jobIndex, ip, job, origin, name, event.Fields["delta"], timestamp))
	case "Error":
		message, err := json.Marshal(event)
		if err == nil {
			msg = message
		}
	case "ContainerMetric":
		FormatTimestamp(event, "timestamp")
		deployment := event.Fields["deployment"]
		jobIndex := event.Fields["job_index"]
		ipString := event.Fields["ip"]
		ip := ""
		if ipString != nil && ipString != "" {
			ip = fmt.Sprintf(" ip=%s", ipString)
		}
		job := event.Fields["job"]
		origin := event.Fields["origin"]
		cfOrgName := event.Fields["cf_org_name"]
		cfOrgId := event.Fields["cf_org_id"]
		cfSpaceName := event.Fields["cf_space_name"]
		cfSpaceId := event.Fields["cf_space_id"]
		cfAppName := event.Fields["cf_app_name"]
		cfAppId := event.Fields["cf_app_id"]
		instanceIndex := event.Fields["instance_index"]
		timestamp := event.Fields["timestamp"]
		msg = []byte(fmt.Sprintf("deployment=%s job_index=%s%s job=%s origin=%s cf_org_name=%s cf_org_id=%s cf_space_name=%s cf_space_id=%s cf_app_name=%s cf_app_id=%s instance_index=%d metric=cpu_percentage  %f %d\n" +
			"deployment=%s job_index=%s%s job=%s origin=%s cf_org_name=%s cf_org_id=%s cf_space_name=%s cf_space_id=%s cf_app_name=%s cf_app_id=%s instance_index=%d metric=disk_bytes  %d %d\n" +
			"deployment=%s job_index=%s%s job=%s origin=%s cf_org_name=%s cf_org_id=%s cf_space_name=%s cf_space_id=%s cf_app_name=%s cf_app_id=%s instance_index=%d metric=disk_bytes_quota  %d %d\n" +
			"deployment=%s job_index=%s%s job=%s origin=%s cf_org_name=%s cf_org_id=%s cf_space_name=%s cf_space_id=%s cf_app_name=%s cf_app_id=%s instance_index=%d metric=memory_bytes  %d %d\n" +
			"deployment=%s job_index=%s%s job=%s origin=%s cf_org_name=%s cf_org_id=%s cf_space_name=%s cf_space_id=%s cf_app_name=%s cf_app_id=%s instance_index=%d metric=memory_bytes_quota  %d %d", 
			deployment, jobIndex, ip, job, origin, cfOrgName, cfOrgId, cfSpaceName, cfSpaceId, cfAppName, cfAppId, instanceIndex, event.Fields["cpu_percentage"], timestamp,
			deployment, jobIndex, ip, job, origin, cfOrgName, cfOrgId, cfSpaceName, cfSpaceId, cfAppName, cfAppId, instanceIndex, event.Fields["disk_bytes"], timestamp,
			deployment, jobIndex, ip, job, origin, cfOrgName, cfOrgId, cfSpaceName, cfSpaceId, cfAppName, cfAppId, instanceIndex, event.Fields["disk_bytes_quota"], timestamp,
			deployment, jobIndex, ip, job, origin, cfOrgName, cfOrgId, cfSpaceName, cfSpaceId, cfAppName, cfAppId, instanceIndex, event.Fields["memory_bytes"], timestamp,
			deployment, jobIndex, ip, job, origin, cfOrgName, cfOrgId, cfSpaceName, cfSpaceId, cfAppName, cfAppId, instanceIndex, event.Fields["memory_bytes_quota"], timestamp))
	}

	buf := new(bytes.Buffer)
	buf.Write(msg)
	result := ""
	for _, message := range strings.Split(buf.String(), "\n") {
		if WantedEvent(message, includeOnlyMatchingFilter, excludeAlwaysMatchingFilter) {
			result += message + "\n"			
		}
	}
	return result
}

func (s *SumoLogicAppender) AppendLogs(buffer *SumoBuffer) {
	event := s.nozzleQueue.Pop().CopyEvent()
	eventString := StringBuilder(event, s.verboseLogMessages, s.includeOnlyMatchingFilter, s.excludeAlwaysMatchingFilter, s.customMetadata)
	if event.Type == "ValueMetric" || event.Type == "CounterEvent" || event.Type == "ContainerMetric" {
		buffer.metricStringToSend.Write([]byte(eventString))
	} else {
		buffer.logStringToSend.Write([]byte(eventString))
	}
	if eventString != "" {
		newLines := strings.Count(eventString, "\n")
		buffer.eventsInCurrentBuffer += newLines
	}
}

func ParseCustomInput(customInput string) map[string][]string {
	cInputArray := strings.Split(customInput, ",")
	customInputMap := make(map[string][]string)
	for i := 0; i < len(cInputArray); i++ {
		entry := strings.Split(cInputArray[i], ":")
		customInputMap[entry[0]] = append(customInputMap[entry[0]], entry[1])
	}
	return customInputMap
}

func (s *SumoLogicAppender) SendToSumo(logStringToSend string, url string, isMetric bool) {
	logging.Trace.Println("Attempting to send to Sumo Endpoint: " + url)
	if logStringToSend != "" {
		var buf bytes.Buffer
		g := gzip.NewWriter(&buf)
		g.Write([]byte(logStringToSend))
		g.Close()
		request, err := http.NewRequest("POST", url, &buf)
		if err != nil {
			logging.Error.Printf("http.NewRequest() error: %v\n", err)
			return
		}
		request.Header.Add("Content-Encoding", "gzip")
		request.Header.Add("X-Sumo-Client", "cloudfoundry-sumologic-nozzle v"+s.nozzleVersion)
		
		if isMetric {
			request.Header.Add("Content-Type", "application/vnd.sumologic.carbon2")
		}
		if s.sumoName != "" {
			request.Header.Add("X-Sumo-Name", s.sumoName)
		}
		if s.sumoHost != "" {
			request.Header.Add("X-Sumo-Host", s.sumoHost)
		}
		if s.sumoCategory != "" {
			request.Header.Add("X-Sumo-Category", s.sumoCategory)
		}
		//checking the timer before first POST intent
		for time.Since(s.timerBetweenPost) < s.sumoPostMinimumDelay {
			logging.Trace.Println("Delaying Post because minimum post timer not expired")
			time.Sleep(100 * time.Millisecond)
		}
		response, err := s.httpClient.Do(request)

		if (err != nil) || (response.StatusCode != 200 && response.StatusCode != 302 && response.StatusCode < 500) {
			logging.Info.Println("Endpoint dropped the post send")
			logging.Info.Println("Waiting for 300 ms to retry")
			time.Sleep(300 * time.Millisecond)
			statusCode := 0
			err := Retry(func(attempt int) (bool, error) {
				var errRetry error
				request, err := http.NewRequest("POST", url, &buf)
				if err != nil {
					logging.Error.Printf("http.NewRequest() error: %v\n", err)
				}
				request.Header.Add("Content-Encoding", "gzip")
				request.Header.Add("X-Sumo-Client", "cloudfoundry-sumologic-nozzle v"+s.nozzleVersion)
				
				if isMetric {
					request.Header.Add("Content-Type", "application/vnd.sumologic.carbon2")
				}
				if s.sumoName != "" {
					request.Header.Add("X-Sumo-Name", s.sumoName)
				}
				if s.sumoHost != "" {
					request.Header.Add("X-Sumo-Host", s.sumoHost)
				}
				if s.sumoCategory != "" {
					request.Header.Add("X-Sumo-Category", s.sumoCategory)
				}
				//checking the timer before POST (retry intent)
				for time.Since(s.timerBetweenPost) < s.sumoPostMinimumDelay {
					logging.Trace.Println("Delaying Post because minimum post timer not expired")
					time.Sleep(100 * time.Millisecond)
				}
				response, errRetry = s.httpClient.Do(request)

				if errRetry != nil {
					logging.Error.Printf("http.Do() error: %v\n", errRetry)
					logging.Info.Println("Waiting for 300 ms to retry after error")
					time.Sleep(300 * time.Millisecond)
					return attempt < 5, errRetry
				} else if response.StatusCode != 200 && response.StatusCode != 302 && response.StatusCode < 500 {
					logging.Info.Println("Endpoint dropped the post send again")
					logging.Info.Println("Waiting for 300 ms to retry after a retry ...")
					statusCode = response.StatusCode
					time.Sleep(300 * time.Millisecond)
					return attempt < 5, errRetry
				} else if response.StatusCode == 200 {
					logging.Trace.Println("Post of logs successful after retry...")
					s.timerBetweenPost = time.Now()
					statusCode = response.StatusCode
					return true, err
				}
				return attempt < 5, errRetry
			})
			if err != nil {
				logging.Error.Println("Error, Not able to post after retry")
				logging.Error.Printf("http.Do() error: %v\n", err)
				return
			} else if statusCode != 200 {
				logging.Error.Printf("Not able to post after retry, with status code: %d", statusCode)
			}
		} else if response.StatusCode == 200 {
			logging.Trace.Println("Post of logs successful")
			s.timerBetweenPost = time.Now()
		}

		if response != nil {
			defer response.Body.Close()
		}
	}
}

//------------------Retry Logic Code-------------------------------

// MaxRetries is the maximum number of retries before bailing.
var MaxRetries = 10
var errMaxRetriesReached = errors.New("exceeded retry limit")

// Func represents functions that can be retried.
type Func func(attempt int) (retry bool, err error)

// Do keeps trying the function until the second argument
// returns false, or no error is returned.
func Retry(fn Func) error {
	var err error
	var cont bool
	attempt := 1
	for {
		cont, err = fn(attempt)
		if !cont || err == nil {
			break
		}
		attempt++
		if attempt > MaxRetries {
			return errMaxRetriesReached
		}
	}
	return err
}

// IsMaxRetries checks whether the error is due to hitting the
// maximum number of retries or not.
func IsMaxRetries(err error) bool {
	return err == errMaxRetriesReached
}
