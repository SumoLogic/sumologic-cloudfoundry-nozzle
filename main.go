package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/SumoLogic/sumologic-cloudfoundry-nozzle/caching"
	"github.com/SumoLogic/sumologic-cloudfoundry-nozzle/eventQueue"
	"github.com/SumoLogic/sumologic-cloudfoundry-nozzle/eventRouting"
	"github.com/SumoLogic/sumologic-cloudfoundry-nozzle/events"
	"github.com/SumoLogic/sumologic-cloudfoundry-nozzle/firehoseclient"
	"github.com/SumoLogic/sumologic-cloudfoundry-nozzle/logging"
	"github.com/SumoLogic/sumologic-cloudfoundry-nozzle/sumoCFFirehose"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	apiEndpoint                 = kingpin.Flag("api_endpoint", "URL to CF API Endpoint").Envar("API_ENDPOINT").String()
	sumoEndpointsString         = kingpin.Flag("sumo_endpoints", "SUMO-ENDPOINTS Complete URLs for the endpoints, copied from the Sumo Logic HTTP Source configuration").Envar("SUMO_ENDPOINTS").String()
	subscriptionId              = kingpin.Flag("subscription_id", "Cloud Foundry ID for the subscription.").Default("firehose").Envar("FIREHOSE_SUBSCRIPTION_ID").String()
	user                        = kingpin.Flag("cloudfoundry_user", "Cloud Foundry User").Envar("CLOUDFOUNDRY_USER").String() //user created in CF, authorized to connect the firehose
	password                    = kingpin.Flag("cloudfoundry_password", "Cloud Foundry Password").Envar("CLOUDFOUNDRY_PASSWORD").String()
	keepAlive, errDt		    = time.ParseDuration("25s") //default Error,ContainerMetric,HttpStart,HttpStop,HttpStartStop,LogMessage,ValueMetric,CounterEvent
	wantedEvents                = kingpin.Flag("events", fmt.Sprintf("Comma separated list of events you would like. Valid options are %s", eventRouting.GetListAuthorizedEventEvents())).Default("LogMessage").Envar("EVENTS").String()
	boltDatabasePath            = "event.db"
	skipSSLValidation           = kingpin.Flag("skip_ssl_validation", "Skip SSL validation (to allow things like self-signed certs). Do not set to true in production").Default("false").Envar("SKIP_SSL_VALIDATION").Bool()
	tickerTime                  = kingpin.Flag("nozzle_polling_period", "How frequently this Nozzle polls the CF Firehose for data").Default("5m").Envar("NOZZLE_POLLING_PERIOD").Duration()
	eventsBatchSize             = kingpin.Flag("log_events_batch_size", "When number of messages in the buffer is equal to this flag, send those to Sumo Logic").Default("500").Envar("LOG_EVENTS_BATCH_SIZE").Int()
	sumoPostMinimumDelay        = kingpin.Flag("sumo_post_minimum_delay", "Minimum time between HTTP POST to Sumo Logic").Default("2000ms").Envar("SUMO_POST_MINIMUM_DELAY").Duration()
	sumoCategory                = kingpin.Flag("sumo_category", "This value overrides the default 'Source Category' associated with the configured Sumo Logic HTTP Source").Default("").Envar("SUMO_CATEGORY").String()
	sumoName                    = kingpin.Flag("sumo_name", "This value overrides the default 'Source Name' associated with the configured Sumo Logic HTTP Source").Default("").Envar("SUMO_NAME").String()
	sumoHost                    = kingpin.Flag("sumo_host", "This value overrides the default 'Source Host' associated with the configured Sumo Logic HTTP Source").Default("").Envar("SUMO_HOST").String()
	verboseLogMessages          = kingpin.Flag("verbose_log_messages", "Enable Verbose in 'LogMessage' Event. If this flag NOT present, the LogMessage will contain ONLY the fields: tiemstamp, cf_app_guid, Msg").Default("false").Envar("VERBOSE_LOG_MESSAGES").Bool()
	customMetadata              = kingpin.Flag("custom_metadata", "Use this flag for addingCustom Metadata to the JSON (key1:value1,key2:value2, etc...)").Default("").Envar("CUSTOM_METADATA").String()
	includeOnlyMatchingFilter   = kingpin.Flag("include_only_matching_filter", "Adds an 'Include only' filter to Events content (key1:value1,key2:value2, etc...)").Default("").Envar("INCLUDE_ONLY_MATCHING_FILTER").String()
	excludeAlwaysMatchingFilter = kingpin.Flag("exclude_always_matching_filter", "Adds an 'Exclude always' filter to Events content (key1:value1,key2:value2, etc...)").Default("").Envar("EXCLUDE_ALWAYS_MATCHING_FILTER").String()
)

var (
	version = "0.2.0"
)

func main() {
	//logging init
	logging.Init(ioutil.Discard, os.Stdout, os.Stdout, os.Stderr)

	kingpin.Version(version)
	kingpin.Parse()

	sumoEndpoints := parseCollectionEndpoints(*sumoEndpointsString)

	logging.Info.Println("Set Configurations:")
	logging.Info.Println("CF API Endpoint: " + *apiEndpoint)
	logging.Info.Printf("Sumo Logic Endpoints: %v", sumoEndpoints)
	logging.Info.Println("Cloud Foundry Nozzle Subscription ID: " + *subscriptionId)
	logging.Info.Println("Cloud Foundry User: " + *user)
	logging.Info.Println("Events Selected: " + *wantedEvents)
	logging.Info.Printf("Skip SSL Validation: %v", *skipSSLValidation)
	logging.Info.Printf("Nozzle Polling Period: %v", *tickerTime)
	logging.Info.Printf("Log Events Batch Size: [%d]", *eventsBatchSize)
	logging.Info.Printf("Sumo Logic HTTP Post Minimum Delay: %v", *sumoPostMinimumDelay)
	if *sumoName != "" {
		logging.Info.Println("Sumo Logic Name: " + *sumoName)
	}
	if *sumoHost != "" {
		logging.Info.Println("Sumo Logic Host: " + *sumoHost)
	}
	if *sumoCategory != "" {
		logging.Info.Println("Sumo Logic Category: " + *sumoCategory)
	}
	logging.Info.Printf("Verbose Log Messages: %v\n", *verboseLogMessages)
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
		os.Exit(1)
	}

	//Creating Caching
	var cachingClient caching.Caching
	if caching.IsNeeded(*wantedEvents) {
		cachingClient = caching.NewCachingBolt(cfClient, boltDatabasePath)
	} else {
		cachingClient = caching.NewCachingEmpty()
	}

	logging.Info.Println("Creating queue")
	queue := eventQueue.NewQueue(make([]*events.Event, 100))
	loggingClientSumo := sumoCFFirehose.NewSumoLogicAppender(sumoEndpoints, 5000, &queue, *eventsBatchSize, *sumoPostMinimumDelay, *sumoCategory, *sumoName, *sumoHost, *verboseLogMessages, *customMetadata, *includeOnlyMatchingFilter, *excludeAlwaysMatchingFilter, version)
	go loggingClientSumo.Start() //multi

	logging.Info.Println("Creating Events")
	events := eventRouting.NewEventRouting(cachingClient, &queue)
	err := events.SetupEventRouting(*wantedEvents)
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

type endpointStruct struct {
	Endpoint	string 	`json:"endpoint"`
    GUID		string 	`json:"guid"`
}

func parseCollectionEndpoints(jsonString string) (target []string) {
	res := []endpointStruct{}
	json.Unmarshal([]byte(jsonString), &res)
	target = make([]string, len(res))
	for i, entry := range res {
		target[i] = entry.Endpoint
	}
	return
}
