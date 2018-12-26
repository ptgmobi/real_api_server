#!/bin/bash

echo "after install start"

region=$(cat ~/.aws/config | grep region | awk -F' = ' '{print $2}')

bucket=""
if [ "$region" == "ap-southeast-1" ]; then
    bucket=cloudmobi-config
fi

echo "after install: bucket: ${bucket}"

if [ ! -d "/pdata1/log/offer" ]; then
    mkdir -p /pdata1/log/offer
fi

pushd /opt/real_api_server

echo "after install: sync: s3://${bucket}/real-api-server/conf/"
aws s3 sync s3://${bucket}/real-api-server/conf/ conf/

echo "make deps"
make deps > build.log 2>&1 || (cat build.log && exit 1)

echo "make"
make > build.log 2>&1 || (cat build.log && exit 1)

echo "ok"
popd
