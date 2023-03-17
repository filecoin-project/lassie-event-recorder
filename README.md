# :movie_camera: Lassie Event Recorder

[![Go Reference](https://pkg.go.dev/badge/github.com/filecoin-project/lassie-event-recorder.svg)](https://pkg.go.dev/github.com/filecoin-project/lassie-event-recorder)
[![Go Test](https://github.com/filecoin-project/lassie-event-recorder/actions/workflows/go-test.yml/badge.svg)](https://github.com/filecoin-project/lassie-event-recorder/actions/workflows/go-test.yml)
> Records Retrieval Events published by [`lassie`](https://github.com/filecoin-project/lassie)

This repo provides a Golang implementation of [`lassie`](https://github.com/filecoin-project/lassie) event recorder.
It exposes a REST HTTP API that accepts `lassie` retrieval events and stores them in the configured postgres database. 


## Install

To install the event recorder run:
```shell
go install github.com/filecoin-project/lassie-event-recorder/cmd/recorder@latest
```

## Usage

```text
$ recorder --help
Usage of recorder:
  -dbDSN string
        The database Data Source Name. Alternatively, it may be specified via LASSIE_EVENT_RECORDER_DB_DSN environment variable. If both are present, the environment variable takes precedence.
  -httpListenAddr string
        The HTTP server listen address in address:port format. (default "0.0.0.0:8080")
  -logLevel string
        The logging level. Only applied if GOLOG_LOG_LEVEL environment variable is unset. (default "info")
```

### Running event recorder locally

To start the recorder service running locally, execute:
```shell
docker-compose up
```

The recorder service is then accessible on `localhost:8080`. 
Here is an example `curl` that posts an event to the server:

```shell
curl -v -X POST --location "http://localhost:8080/v1/retrieval-events" \
    -H "Content-Type: application/json" \
    -d "{
          \"events\": [
            {
              \"retrievalId\": \"d05a522e-9608-401a-a33f-1eb4ac4111a0\",
              \"instanceId\": \"test-instance-id\",
              \"cid\": \"bafybeic4jpi2detp5n3q6rjo7ckulebtr7dsvt2tbrtcqlnnzqmi3bzz2y\",
              \"storageProviderId\": \"12D3KooWHwRmkj4Jxfo2YnKJC4YBzTNEGDW6Et4E68r7RYVXk46h\",
              \"phase\": \"query\",
              \"phaseStartTime\": \"2023-03-02T00:15:50.479910907Z\",
              \"eventName\": \"started\",
              \"eventTime\": \"2023-03-02T00:15:52.361371026Z\"
            }
          ]
        }"
```

To stop the containers press `Ctrl + C`.

#### Postgres

The postgres container can be accessed via port `localhost:5432` for local DBMS setups or psql connections.

#### Lassie

Lassie can talk to this local event recorder instance by using the `--endpoint-url` and `--endpoint-instance-id` options on either the `daemon`

## Resources
 - [JS implementation of lassie event recorder](https://github.com/filecoin-project/autoretrieve-deploy/tree/19f55fad23555add12e312ee20e0f54383f8482c/lassie-event-recorder-api)
 - [K8S manifests to deploy event recorder as a service](https://github.com/filecoin-project/autoretrieve-deploy/tree/main/deploy/manifests/base/lassie-event-recorder)
   - [`dev` manifests deployed using `nginx` ingress with basic auth enabled](https://github.com/filecoin-project/autoretrieve-deploy/tree/main/deploy/manifests/dev/us-east-2/lassie-event-recorder)

## License
[SPDX-License-Identifier: Apache-2.0 OR MIT](LICENSE.md)
