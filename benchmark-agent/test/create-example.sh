#!/bin/bash

curl -X POST -H "Content-Type: application/json" \
-d "{ \
      \"name\": \"busycpu\", \
      \"count\": 4, \
      \"resources\": { \
        \"cpushares\": 512
      }, \
      \"image\": \"hyperpilot\/busycpu\" \
    }" \
http://localhost:7778/benchmarks
