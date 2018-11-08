#!/bin/bash
echo "*************************************************************************"
echo "Removing product and release folder. Then build sumo-logic-nozzle.zip"
echo "*************************************************************************"
rm -rf product release
zip -r sumo-logic-nozzle.zip bitbucket-pipelines.yml caching/ ci/ eventQueue/ eventRouting/ events/ firehoseclient/ LICENSE logging/ main.go manifest.yml event.db Procfile sumoCFFirehose/ utils/ vendor/
