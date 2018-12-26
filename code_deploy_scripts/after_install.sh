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

pushd /opt/offer_server

echo "after install: sync: s3://${bucket}/offer-server/conf/"
aws s3 sync s3://${bucket}/offer-server/conf/ conf/

apple_ip_filter_dir="/pdata1/offer_server_data/apple_ip_filter"
if [ ! -d "${apple_ip_filter_dir}" ]; then
    mkdir -p ${apple_ip_filter_dir}
fi

apple_pmt_dir="/pdata1/offer_server_data/bundles"
if [ ! -d "${apple_pmt_dir}" ]; then
    mkdir -p ${apple_pmt_dir}
fi

echo "after install: sync: s3://${bucket}/apple_bloom_filter/data/"
aws s3 sync s3://${bucket}/apple_bloom_filter/data/ ${apple_ip_filter_dir}

bf_md5=$(md5sum "${apple_ip_filter_dir}/apple_bloom_filter" | cut -d ' ' -f1)
if [ "$bf_md5" != "9fe1a45f115e5fb8d049257f494c2020" ]; then
    echo "md5 un-match, apple_ip_bloom_filter changed"
    exit 1
fi

replace_pkg_name_dir="/pdata1/offer_server_data/replace_pkg_name"
if [ ! -d "${replace_pkg_name_dir}" ]; then
    mkdir -p ${replace_pkg_name_dir}
fi

echo "after install: sync: s3://${bucket}/offer-server/replace_pkg_name/"
aws s3 sync s3://${bucket}/offer-server/replace_pkg_name/ ${replace_pkg_name_dir}


echo "make deps"
make deps > build.log 2>&1 || (cat build.log && exit 1)

echo "make"
make > build.log 2>&1 || (cat build.log && exit 1)

echo "ok"
popd
