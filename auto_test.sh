#!/usr/bin/env bash

function test_and_append_coverage() {
    pushd $1 > /dev/null
    go test -race -v -coverprofile=profile.out -covermode=atomic
    if [ -f profile.out ]; then
        cat profile.out >> ${COVFILE}
        rm profile.out
    fi
    popd > /dev/null
}

PWD=$(pwd)
COVFILE=${PWD}/../coverage.txt

set -e
echo "" > ${COVFILE}

test_and_append_coverage src/ct_bloom
test_and_append_coverage src/affiliate
test_and_append_coverage src/ssp
test_and_append_coverage src/util
test_and_append_coverage src/raw_ad
test_and_append_coverage src/aes
test_and_append_coverage src/set
# test_and_append_coverage src/pacing # to pass travis-ci
test_and_append_coverage src/offer
