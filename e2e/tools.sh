#!/usr/bin/env bash

TARGET_ADDR=${TARGET_ADDR:-http://localhost:9898/headers}
DURATION=${DURATION:-10s}
RATE=${RATE:-10}
VEGETA_EXTRA_FLAGS=

# Exit on Ctrl-C
trap "exit" INT

# Stop all the sub-processes on exit
trap 'if [ ! -z "$(jobs -p)" ]; then echo Stopping all sub-processes 1>&2 ; kill $(jobs -p); fi' EXIT

#x-cluster-id: 123
encode() {
echo "GET ${TARGET_ADDR}
" | \
  vegeta attack -rate ${RATE} -duration ${DURATION} ${VEGETA_EXTRA_FLAGS} | \
  vegeta encode
}

# fromjson: See https://stackoverflow.com/questions/35154684/use-jq-to-parse-a-json-string
encode_parse() {
 encode | parse
}

parse() {
  jq --unbuffered -Mc '.body = (.body|@base64d|fromjson|.["X-Cluster-Id"] | first | tonumber)'
}

aggregate() {
  jaggr @count=rps hist\[123,456\]:body hist\[100,200,300,400,500\]:code p25,p50,p95:latency sum:bytes_in sum:bytes_out
}

encode_parse_jaggr() {
  encode | parse | aggregate
}

encode_aggregate() {
  encode | \
  jaggr @count=rps hist\[100,200,300,400,500\]:code p25,p50,p95:latency sum:bytes_in sum:bytes_out
}

jplot() {
  exec jplot rps+body.hist.123+body.hist.456 rps+code.hist.100+code.hist.200+code.hist.300+code.hist.400+code.hist.500 latency.p95+latency.p50+latency.p25 bytes_in.sum+bytes_out.sum
}

"$@"
