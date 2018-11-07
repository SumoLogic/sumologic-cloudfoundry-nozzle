package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/SumoLogic/sumologic-cloudfoundry-nozzle/caching"
	"github.com/SumoLogic/sumologic-cloudfoundry-nozzle/eventQueue"
	"github.com/SumoLogic/sumologic-cloudfoundry-nozzle/eventRouting"
	"github.com/SumoLogic/sumologic-cloudfoundry-nozzle/events"
	"github.com/SumoLogic/sumologic-cloudfoundry-nozzle/firehoseclient"
	"github.com/SumoLogic/sumologic-cloudfoundry-nozzle/logging"
	"github.com/SumoLogic/sumologic-cloudfoundry-nozzle/sumoCFFirehose"
	"github.com/cloudfoundry-community/go-cfclient"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	apiEndpoint         = kingpin.Flag("api_endpoint", "URL to CF API Endpoint").Envar("API_ENDPOINT").String()
	sumoEndpointsString = kingpin.Flag("sumo_endpoints", "SUMO-ENDPOINTS Complete URLs for the endpoints, copied from the Sumo Logic HTTP Source configuration").Envar("SUMO_ENDPOINTS").String()
	subscriptionId      = kingpin.Flag("subscription_id", "Cloud Foundry ID for the subscription.").Default("firehose").Envar("FIREHOSE_SUBSCRIPTION_ID").String()
	user                = kingpin.Flag("cloudfoundry_user", "Cloud Foundry User").Envar("CLOUDFOUNDRY_USER").String() //user created in CF, authorized to connect the firehose
	password            = kingpin.Flag("cloudfoundry_password", "Cloud Foundry Password").Envar("CLOUDFOUNDRY_PASSWORD").String()
	keepAlive, errDt    = time.ParseDuration("25s") //default Error,ContainerMetric,HttpStart,HttpStop,HttpStartStop,LogMessage,ValueMetric,CounterEvent
	wantedEvents        = kingpin.Flag("events", fmt.Sprintf("Comma separated list of events you would like. Valid options are %s", eventRouting.GetListAuthorizedEventEvents())).Default("LogMessage").Envar("EVENTS").String()
	boltDatabasePath    = "event.db"
	skipSSLValidation   = kingpin.Flag("skip_ssl_validation", "Skip SSL validation (to allow things like self-signed certs). Do not set to true in production").Default("false").Envar("SKIP_SSL_VALIDATION").Bool()
	tickerTime          = kingpin.Flag("nozzle_polling_period", "How frequently this Nozzle polls the CF Firehose for data").Default("5m").Envar("NOZZLE_POLLING_PERIOD").Duration()
	eventsBatchSize     = kingpin.Flag("log_events_batch_size", "When number of messages in the buffer is equal to this flag, send those to Sumo Logic").Default("500").Envar("LOG_EVENTS_BATCH_SIZE").Int()
	verboseLogMessages  = kingpin.Flag("verbose_log_messages", "Enable Verbose in 'LogMessage' Event. If this flag NOT present, the LogMessage will contain ONLY the fields: tiemstamp, cf_app_guid, Msg").Default("true").Envar("VERBOSE_LOG_MESSAGES").Bool()
)

var (
	version = "0.9.1"
)

func main() {
	//logging init
	logging.Init(ioutil.Discard, os.Stdout, os.Stdout, os.Stderr)

	kingpin.Version(version)
	kingpin.Parse()

	sumoConfigs, err := parseSumoConfigs(*sumoEndpointsString)
	if err != nil {
		logging.Error.Fatal("Error parsing sumo configs: ", err.Error())
	}
	cfApi := parseCfApiFromVcapApplication(os.Getenv("VCAP_APPLICATION"))
	if *apiEndpoint == "" {
		logging.Info.Println("Cloud Foundry API Endpoint was empty. Setting it to cf_api value: " + cfApi)
		*apiEndpoint = cfApi
	}

	logging.Info.Println("Set Configurations:")
	logging.Info.Println("cf_api: " + cfApi)
	logging.Info.Println("CF API Endpoint: " + *apiEndpoint)
	logging.Info.Println("Cloud Foundry Nozzle Subscription ID: " + *subscriptionId)
	logging.Info.Println("Cloud Foundry User: " + *user)
	logging.Info.Println("Events Selected: " + *wantedEvents)
	logging.Info.Printf("Skip SSL Validation: %v", *skipSSLValidation)
	logging.Info.Printf("Nozzle Polling Period: %v", *tickerTime)
	logging.Info.Printf("Log Events Batch Size: [%d]", *eventsBatchSize)
	logging.Info.Printf("Verbose Log Messages: %v\n", *verboseLogMessages)
	logging.Info.Printf("Sumo Logic Configurations: %v", sumoConfigs)
	logging.Info.Println("Starting Sumo Logic Nozzle " + version)

	if errDt != nil {
		logging.Info.Println("Could not parse Duration...")
	}

	c := cfclient.Config{
		ApiAddress:        *apiEndpoint,
		Username:          *user,
		Password:          *password,
		SkipSslValidation: *skipSSLValidation,
	}
	cfClient, errCfClient := cfclient.NewClient(&c)

	if errCfClient != nil {
		logging.Error.Fatal("Error setting up CF Client: ", errCfClient)
	}

	//Creating Caching
	var cachingClient caching.Caching
	if caching.IsNeeded(*wantedEvents) {
		cachingClient = caching.NewCachingBolt(cfClient, boltDatabasePath)
	} else {
		cachingClient = caching.NewCachingEmpty()
	}

	queues := make([]*eventQueue.Queue, len(sumoConfigs))
	for i, sumoConfig := range sumoConfigs {
		logging.Info.Println("Creating queue for endpoint: " + sumoConfig.Endpoint)
		queue := eventQueue.NewQueue(make([]*events.Event, 100))
		queues[i] = &queue
		loggingClientSumo := sumoCFFirehose.NewSumoLogicAppender(sumoConfig.Endpoint, 5000, &queue, *eventsBatchSize, sumoConfig.PostMinimumDelay, sumoConfig.Category, sumoConfig.Name, sumoConfig.Host, *verboseLogMessages, sumoConfig.CustomMetadata, sumoConfig.IncludeOnlyMatchingFilter, sumoConfig.ExcludeAlwaysMatchingFilter, version)
		go loggingClientSumo.Start() //multi
	}

	logging.Info.Println("Creating Events")
	events := eventRouting.NewEventRouting(cachingClient, queues)
	err = events.SetupEventRouting(*wantedEvents)
	if err != nil {
		logging.Error.Fatal("Error setting up event routing: ", err)
		os.Exit(1)
	}

	// Parse extra fields from cmd call
	cachingClient.CreateBucket()
	//Let's Update the database the first time
	logging.Info.Printf("Start filling app/space/org cache.\n")
	apps := cachingClient.GetAllApp()
	logging.Info.Printf("Done filling cache! Found [%d] Apps \n", len(apps))

	logging.Info.Println("Apps found: ")
	for i := 0; i < len(apps); i++ {
		logging.Info.Printf("[%d] "+apps[i].Name+" GUID: "+apps[i].Guid, i+1)
	}
	cachingClient.PerformPoollingCaching(*tickerTime)

	firehoseConfig := &firehoseclient.FirehoseConfig{
		TrafficControllerURL:   cfClient.Endpoint.DopplerEndpoint,
		InsecureSSLSkipVerify:  *skipSSLValidation,
		IdleTimeoutSeconds:     keepAlive,
		FirehoseSubscriptionID: *subscriptionId,
	}

	logging.Info.Printf("Connecting to Firehose... \n")
	firehoseClient := firehoseclient.NewFirehoseNozzle(cfClient, events, firehoseConfig)
	errFirehose := firehoseClient.Start()
	logging.Info.Printf("FirehoseClient Error: %v", errFirehose)
	defer cachingClient.Close()

}

type sumoConfigStruct struct {
	Endpoint                    string        `json:"endpoint"`
	PostMinimumDelay            time.Duration `json:"sumo_post_minimum_delay"`
	Category                    string        `json:"sumo_category"`
	Name                        string        `json:"sumo_name"`
	Host                        string        `json:"sumo_host"`
	CustomMetadata              string        `json:"custom_metadata"`
	IncludeOnlyMatchingFilter   string        `json:"include_only_matching_filter"`
	ExcludeAlwaysMatchingFilter string        `json:"exclude_always_matching_filter"`
	GUID                        string        `json:"guid"`
}

func (s sumoConfigStruct) String() string {
	return fmt.Sprintf("\n"+
		"Sumo Logic Endpoint: %v\n"+
		"Sumo Logic HTTP Post Minimum Delay: %v\n"+
		"Sumo Logic Name: %v\n"+
		"Sumo Logic Host: %v\n"+
		"Sumo Logic Category: %v\n"+
		"Custom Metadata: %v\n"+
		"Include Only Matching Filter: %v\n"+
		"Exclude Always Matching Filter: %v\n",
		s.Endpoint,
		s.PostMinimumDelay,
		s.Name,
		s.Host,
		s.Category,
		s.CustomMetadata,
		s.IncludeOnlyMatchingFilter,
		s.ExcludeAlwaysMatchingFilter)
}

func parseSumoConfigs(jsonString string) ([]sumoConfigStruct, error) {
	res := []sumoConfigStruct{}
	err := json.Unmarshal([]byte(jsonString), &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

type vcapApplication struct {
	ApplicationId      string `json:"application_id"`
	ApplicationName    string `json:"application_name"`
	ApplicationUris    string `json:"application_uris"`
	ApplicationVersion string `json:"application_version"`
	CfApi              string `json:"cf_api"`
	Host               string `json:"host"`
	Limits             string `json:"limits"`
	Name               string `json:"name"`
	SpaceId            string `json:"space_id"`
	SpaceName          string `json:"space_name"`
	Start              string `json:"start"`
	StartedAt          string `json:"started_at"`
	StartedAtTimestamp string `json:"started_at_timestamp"`
	StateTimestamp     string `json:"state_timestamp"`
	Uris               string `json:"uris"`
	Users              string `json:"users"`
	Version            string `json:"version"`
}

func parseCfApiFromVcapApplication(jsonString string) string {
	res := vcapApplication{}
	json.Unmarshal([]byte(jsonString), &res)
	return res.CfApi
}
