#!/bin/bash

export GOOS=linux; go build .
mv moodle ./artifacts/moodle
docker build -t lmecld/moodle-operator:latest ./artifacts/
rm ./artifacts/moodle



