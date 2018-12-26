#!/bin/bash

source /etc/profile # add FlameGraph path here

function draw_cpu_graph() {
    go tool pprof -seconds=30 -raw -output=offer-cpu.perf http://127.0.0.1:19991/debug/pprof/profile
    stackcollapse-go.pl offer-cpu.perf > offer-cpu.fold
    flamegraph.pl --title="offer server cpu online graph" --colors hot offer-cpu.fold > offer-cpu.svg
}

function draw_mem_graph() {
    go tool pprof -alloc_space -raw -output=offer-mem.perf http://127.0.0.1:19991/debug/pprof/heap
    stackcollapse-go.pl offer-mem.perf > offer-mem.fold
    flamegraph.pl --title="offer server mem graph" --colors mem offer-mem.fold > offer-mem.svg
}

draw_mem_graph
draw_cpu_graph
