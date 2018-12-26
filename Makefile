
RM ?= rm -rf
GOBUILD = go build
GOTEST = go test
GOGET = go get -u
MASTER = bin/tmaster
WORKER = bin/tworker
GUARD = bin/tguard

VARS=vars.mk
$(shell ./build_config ${VARS})
include ${VARS}

.PHONY: main deps test bench clean

main:
	${GOBUILD} -o ${GUARD} src/tworker_guard.go
	${GOBUILD} -o ${WORKER} src/worker.go

deps:
	${GOGET} github.com/aws/aws-sdk-go
	${GOGET} github.com/brg-liuwei/gotools
	${GOGET} github.com/brg-liuwei/godnf
	${GOGET} github.com/garyburd/redigo/redis
	${GOGET} github.com/willf/bloom
	${GOGET} github.com/cloudadrd/amqp
	${GOGET} github.com/cloudadrd/vast

test:
	./auto_test.sh

bench:
	@pushd src/util > /dev/null && ${GOTEST} -bench=. && popd > /dev/null

clean:
	${RM} ${VARS} bin/*
