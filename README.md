# sumologic-cloudfoundry-nozzle

This Nozzle aggregates all the events from the _Firehose_ feature in Cloud Foundry towards Sumo Logic

### Options of use

```
usage: main [<flags>]

Flags:  (See run command, in this document, for syntax of flags)
--help                              Show context-sensitive help (also try --help-long and --help-man).
--api_endpoint=                     CF API Endpoint
--subscription_id="firehose"        Cloud Foundry ID for the subscription.
--cloudfoundry_user=                Cloud Foundry User
--cloudfoundry_password=            Cloud Foundry Password
--events="LogMessage"               Comma separated list of events you would like. Valid options are ContainerMetric, CounterEvent, Error, HttpStart, HttpStartStop,
                                    HttpStop, LogMessage, ValueMetric
--skip_ssl_validation               Skip SSL validation (to allow things like self-signed certs). Do not set to true in production
--nozzle_polling_period=15s         How frequently this Nozzle polls the CF Firehose for data
--log_events_batch_size=500         When number of messages in the buffer is equal to this flag, send those to Sumo Logic
--verbose_log_messages              Enable Verbose in 'LogMessage' Event. If this flag is NOT present, the LogMessage will contain ONLY the fields: tiemstamp, cf_app_guid, Msg
--include_only_matching_filter=""   Adds an 'Include only' filter to Events content (key1:value1,key2:value2, etc...)
--exclude_always_matching_filter="" Adds an 'Exclude always' filter to Events content (key1:value1,key2:value2, etc...)
--version                           Show application version.

```

Also for each endpoint JSON object, the following keys can be defined: 
```
"sumo_endpoint":"<SUMO_HTTP_ENDPOINT>"                    SUMO-ENDPOINT Complete URL for the endpoint, copied from the Sumo Logic HTTP Source configuration
"sumo_post_minimum_delay":"2000ms"    Minimum time between HTTP POST to Sumo Logic
"sumo_category":""                  This value overrides the default 'Source Category' associated with the configured Sumo Logic HTTP Source
"sumo_name":""                      This value overrides the default 'Source Name' associated with the configured Sumo Logic HTTP Source
"sumo_host":""                      This value overrides the default 'Source Host' associated with the configured Sumo Logic HTTP Source
"custom_metadata":""                Use this flag for addingCustom Metadata to the JSON (key1:value1,key2:value2, etc...)
```

### Supported Event type
| Firehose event type | Description                                                                                    |
|---------------------|------------------------------------------------------------------------------------------------|
| **HttpStartStop**   | An HttpStartStop event comprehensively represents the lifecycle of an HTTP request             |
| **LogMessage**      | A Log messages emitted by the application to either stderr or stdout.                          |
| **ContainerMetric** | A ContainerMetrics capture resource utilization for applications running in Garden containers. |
| **CounterEvent**    | A CounterEvents to represent incrementing counters.                                            |
| **ValueMetric**     | A ValueMetrics to represent the instantaneous value of a metric.                               |
| **Error**           | An Error event signifies an error occurring within the originating process.                    |

There are 3 ways to run this Nozzle:

1. Run as standalone app
2. Run as a tile in Pivotal Cloud Foundry
3. Push as an app in a Clod Foundry instance

### Run as standalone app

This is an example for running the Nozzle using the flags options described above:
```
go run main.go --sumo_endpoints='[{"endpoint":"https://sumo-endpoint","sumo_post_minimum_delay": "200ms","sumo_host":"123.123.123.0", "sumo_category":"categoryTest", "sumo_name":"NameTestMETA", "include_only_matching_filter": "","exclude_always_matching_filter": "" }]' --api_endpoint=https://api.endpoint --cloudfoundry_user=some_user --cloudfoundry_password=some_password  --log_events_batch_size=200 --events=LogMessage,ValueMetric,ContainerMetric,CounterEvent,Error,HttpStart,HttpStop --verbose_log_messages
```

If everything goes right, you should see in your terminal the _Nozzle's Logs_ and, in the __Sumo Logic endpoint__ (defined in the _--sumo-endpoint_ flag) you should see the logs according the events you choose (_'LogMessage'_ and _'ValueMetric'_ with _verbose_ in this case).


### Filtering Option

It works this way:
* **Case 1**:
**Include-Only filter**="" (_Empty_)
**Exclude-Always filter**="" (_Empty_)
In this case, _**all**_ the events will be sent to Sumo Logic

* **Case 2**:
**Include-Only filter**="" (Empty)
**Exclude-Always filter**= source_type:other,origin:rep
in this case, all the events that contains a _**source-type:other**_ field OR an _**origin:rep**_ field will be not sent to Sumo Logic

* **Case 3**:
**Include-Only filter**=job:diego_cell,source_type:other
**Exclude-Always filter**="" (Empty)
in this case, Only the events that contains a _**job:diego-cell**_ field OR a _**source-type:other**_ field will be sent to Sumo Logic

* **Case 4**:
**Include-Only filter**=job:diego_cell,source_type:other
**Exclude-Always filter**=source_type:app,origin:rep
In this case, all the events that contains a _**job:diego-cell**_ field OR a _**source-type:other**_ field will be sent to Sumo Logic
**AND also**
All the events that contains a _**source-type:other**_ field OR an _**origin:app**_ field will be not sent to Sumo Logic.

**IMPORTANT**: **Exclude filter _overrides_ Include filter** . This way if one or more of the App's logs fields **match both filters** (contains a _Include-Only filter_ field and a _Exclude-Always_ filter), this log will be **NOT** sent to Sumo Logic.


The correct way of using those flags will be something like this:

```
go run main.go --sumo-endpoint=[{"endpoint":"https://sumo-endpoint","sumo_post_minimum_delay":"200ms","include_only_matching_filter":"job:diego_cell,source_type:app","exclude_always_matching_filter":"source_type:other,unit:count"}] --api-endpoint=https://api.endpoint --skip-ssl-validation --cloudfoundry-user=some_user --cloudfoundry-password=some_password  --log-events-batch-size=200 --events=LogMessage,ValueMetric   
```


### Run as a tile in Pivotal Cloud Foundry

**Pivotal Cloud Foundry** (PCF) has a Tile Generator tool which will help you to deploy this Nozzle in PCF allowing an easy configuration of it.

The tile configuration is handled in the 'tile.yml' file. (If you want to modify this file, is worth to mention that it is directly related to the Nozzle flags mentioned before).

#### Steps to run as this Nozzle as a tile in PCF:

##### Step 1 - Install the tile-generator python package
* Follow the Official Pivotal Instructions: http://docs.pivotal.io/tiledev/tile-generator.html#how-to
(only until half of the _step 3_, DON'T DO 'tile init', only cd into the 'sumologic-cloudfoundry-nozzle' folder)

##### Step 2 - Check the tile file

* If you want to add more settings to the tile or remove some. Check the Official Pivotal Documentation for more options http://docs.pivotal.io/tiledev/tile-generator.html#define

##### Step 3 - Prepare your code:

* Zip your entire code and place the zip file into the root directory of the project for which you wish to create a tile. For this tile use this command: (you should do this in a new terminal window)

    ```
    zip -r sumo-logic-nozzle.zip bitbucket-pipelines.yml caching/ ci/ eventQueue/ eventRouting/ events/ firehoseclient/  LICENSE logging/ main.go  Procfile sumoCFFirehose/ utils/ vendor/
    ```
##### Step 4 - Build tile file
* go to the 'tile-generator' terminal window and run

    ```
    $ tile build
    ```
##### Step 5 - Install the tile in Pivotal Cloud Foundry
* Login with proper credentials into the OPS Manager and import the .pivotal file created above and wait.
* Then add it to the Installation Dashboard to configure it. You should able to configure the settings created in the tile file.
* Update the changes and you should start to see some logs in the Sumo Logic Endpoint defined.

### Push as an app in a Cloud Foundry instance

Step 1 - Download the latest release of sumologic-cloudfoundry-nozzle.
```
$ git clone https://github.com/SumoLogic/sumologic-cloudfoundry-nozzle.git
$ cd sumologic-cloudfoundry-nozzle
```

Step 2 - Utilize the CF cli to authenticate with your PCF instance.
```
$ cf login -a https://api.[your cf system domain] -u [your id] --skip-ssl-validation
```
Step 3 - Push sumologic-cloudfoundry-nozzle
```
$ cf push sumologic-cloudfoundry-nozzle --no-start
```
Step 4 - Set environment variables with cf cli or in the https://github.com/SumoLogic/sumologic-cloudfoundry-nozzle/blob/master/manifest.yml. Example:
```
$ cf set-env sumologic-cloudfoundry-nozzle API_ENDPOINT https://api.[your cf system domain]
$ cf set-env sumologic-cloudfoundry-nozzle SUMO_ENDPOINT https://sumo-endpoint
$ cf set-env sumologic-cloudfoundry-nozzle FIREHOSE_SUBSCRIPTION_ID sumologic-cloudfoundry-nozzle
$ cf set-env sumologic-cloudfoundry-nozzle CLOUDFOUNDRY_USER [your doppler.firehose enabled user]
$ cf set-env sumologic-cloudfoundry-nozzle CLOUDFOUNDRY_PASSWORD [your doppler.firehose enabled user password]
$ cf set-env sumologic-cloudfoundry-nozzle EVENTS LogMessage
$ cf set-env sumologic-cloudfoundry-nozzle NOZZLE_POLLING_PERIOD 15s
$ cf set-env sumologic-cloudfoundry-nozzle LOG_EVENTS_BATCHSIZE  200
$ cf set-env sumologic-cloudfoundry-nozzle SUMO_POST_MINIMUM_DELAY 200ms
$ cf set-env sumologic-cloudfoundry-nozzle SUMO_CATEGORY ExampleCategory
$ cf set-env sumologic-cloudfoundry-nozzle SUMO_NAME ExampleName
$ cf set-env sumologic-cloudfoundry-nozzle SUMO_HOST [some ip]
$ cf set-env sumologic-cloudfoundry-nozzle VERBOSE_LOG_MESSAGES  false
$ cf set-env sumologic-cloudfoundry-nozzle CUSTOM_METADATA customData1:customValue1,CustomData2:CustomValue2
$ cf set-env sumologic-cloudfoundry-nozzle INCLUDE_ONLY_MATCHING_FILTER ""
$ cf set-env sumologic-cloudfoundry-nozzle EXCLUDE_ALWAYS_MATCHING_FILTER ""
```

Step 5 - Turn off the health check if you're staging to Diego.

```
$ cf set-health-check sumologic-cloudfoundry-nozzle none
```

Step 6 - Push the app.
```
$ cf push sumologic-cloudfoundry-nozzle --no-route
```
## Authors

mcplusa.com

## Related Sources

* Firehose-to-Syslog Nozzle

### TLS 1.2 Requirement

Sumo Logic only accepts connections from clients using TLS version 1.2 or greater. To utilize the content of this repo, ensure that it's running in an execution environment that is configured to use TLS 1.2 or greater.
