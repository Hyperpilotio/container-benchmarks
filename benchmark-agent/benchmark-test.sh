#!/bin/bash

curl -H "Content-Type: application/json"  -X POST localhost:7778/benchmarks --data @cpu-benchmark.json
curl -H "Content-Type: application/json"  -X POST localhost:7778/benchmarks --data @memcap-benchmark.json

curl -H "Content-Type: application/json"  -X GET localhost:7778/benchmarks/cpu
curl -H "Content-Type: application/json"  -X GET localhost:7778/benchmarks/memCap

curl -H "Content-Type: application/json"  -X DELETE localhost:7778/benchmarks/cpu
curl -H "Content-Type: application/json"  -X DELETE localhost:7778/benchmarks/memCap

curl -H "Content-Type: application/json"  -X PUT localhost:7778/benchmarks/cpu/intensity --data @update-intensity.json
