#!/usr/bin/env bash

docker build . -t rest-api-go
docker run -i -t -p 8080:8080 rest-api-go