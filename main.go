package main

import (
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
	apiEndpoint                 = kingpin.Flag("api-endpoint", "URL to CF API Endpoint").OverrideDefaultFromEnvar("API_ENDPOINT").String()
	sumoEndpoint                = kingpin.Flag("sumo-endpoint", "SUMO-ENDPOINT Complete URL for the endpoint, copied from the Sumo Logic HTTP Source configuration").OverrideDefaultFromEnvar("SUMO_ENDPOINT").String()
	subscriptionId              = kingpin.Flag("subscription-id", "Cloud Foundry ID for the subscription.").Default("firehose").OverrideDefaultFromEnvar("FIREHOSE_SUBSCRIPTION_ID").String()
	user                        = kingpin.Flag("cloudfoundry-user", "Cloud Foundry User").OverrideDefaultFromEnvar("CLOUDFOUNDRY_USER").String() //user created in CF, authorized to connect the firehose
	password                    = kingpin.Flag("cloudfoundry-password", "Cloud Foundry Password").OverrideDefaultFromEnvar("CLOUDFOUNDRY_PASSWORD").String()
	keepAlive, errK             = time.ParseDuration("25s") //default Error,ContainerMetric,HttpStart,HttpStop,HttpStartStop,LogMessage,ValueMetric,CounterEvent
	wantedEvents                = kingpin.Flag("events", fmt.Sprintf("Comma separated list of events you would like. Valid options are %s", eventRouting.GetListAuthorizedEventEvents())).Default("LogMessage").OverrideDefaultFromEnvar("EVENTS").String()
	boltDatabasePath            = "event.db"
	skipSSLValidation           = kingpin.Flag("skip-ssl-validation", "Skip SSL validation (to allow things like self-signed certs). Do not set to true in production").Default("false").OverrideDefaultFromEnvar("SKIP_SSL_VALIDATION").Bool()
	tickerTime                  = kingpin.Flag("nozzle-polling-period", "How frequently this Nozzle polls the CF Firehose for data").Default("15s").OverrideDefaultFromEnvar("NOZZLE_POLLING_PERIOD").Duration()
	eventsBatchSize             = kingpin.Flag("log-events-batch-size", "When number of messages in the buffer is equal to this flag, send those to Sumo Logic").Default("500").OverrideDefaultFromEnvar("LOG_EVENTS_BATCH_SIZE").Int()
	sumoPostMinimumDelay        = kingpin.Flag("sumo-post-minimum-delay", "Minimum time between HTTP POST to Sumo Logic").Default("2000ms").OverrideDefaultFromEnvar("SUMO_POST_MINIMUM_DELAY").Duration()
	sumoCategory                = kingpin.Flag("sumo-category", "This value overrides the default 'Source Category' associated with the configured Sumo Logic HTTP Source").Default("").OverrideDefaultFromEnvar("SUMO_CATEGORY").String()
	sumoName                    = kingpin.Flag("sumo-name", "This value overrides the default 'Source Name' associated with the configured Sumo Logic HTTP Source").Default("").OverrideDefaultFromEnvar("SUMO_NAME").String()
	sumoHost                    = kingpin.Flag("sumo-host", "This value overrides the default 'Source Host' associated with the configured Sumo Logic HTTP Source").Default("").OverrideDefaultFromEnvar("SUMO_HOST").String()
	verboseLogMessages          = kingpin.Flag("verbose-log-messages", "Enable Verbose in 'LogMessage' Event. If this flag NOT present, the LogMessage will contain ONLY the fields: tiemstamp, cf_app_guid, Msg").Default("false").OverrideDefaultFromEnvar("VERBOSE_LOG_MESSAGES").Bool()
	customMetadata              = kingpin.Flag("custom-metadata", "Use this flag for addingCustom Metadata to the JSON (key1:value1,key2:value2, etc...)").Default("").OverrideDefaultFromEnvar("CUSTOM_METADATA").String()
	includeOnlyMatchingFilter   = kingpin.Flag("include-only-matching-filter", "Adds an 'Include only' filter to Events content (key1:value1,key2:value2, etc...)").Default("").OverrideDefaultFromEnvar("INCLUDE_ONLY_MATCHING_FILTER").String()
	excludeAlwaysMatchingFilter = kingpin.Flag("exclude-always-matching-filter", "Adds an 'Exclude always' filter to Events content (key1:value1,key2:value2, etc...)").Default("").OverrideDefaultFromEnvar("EXCLUDE_ALWAYS_MATCHING_FILTER").String()
)

var (
	version = "0.1.0"
)

func main() {
	//logging init
	logging.Init(ioutil.Discard, os.Stdout, os.Stdout, os.Stderr)

	kingpin.Version(version)
	kingpin.Parse()

	logging.Info.Println("Set Configurations:")
	logging.Info.Println("CF API Endpoint: " + *apiEndpoint)
	logging.Info.Println("Sumo Logic Endpoint: " + *sumoEndpoint)
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
	loggingClientSumo := sumoCFFirehose.NewSumoLogicAppender(*sumoEndpoint, 5000, &queue, *eventsBatchSize, *sumoPostMinimumDelay, *sumoCategory, *sumoName, *sumoHost, *verboseLogMessages, *customMetadata, *includeOnlyMatchingFilter, *excludeAlwaysMatchingFilter, version)
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
