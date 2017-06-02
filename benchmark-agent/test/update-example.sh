#!/bin/bash

curl -X PUT -H "Content-Type: application/json" \
-d "{ \
      \"cpushares\": 256 \
    }" \
http://localhost:7778/benchmarks/busycpu/resources
