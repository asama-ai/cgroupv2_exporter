# cgroupv2_exporter
Prometheus exporter for Cgroup v2 metrics, written in Go with pluggable metric collectors similar to [node_exporter](https://github.com/prometheus/node_exporter).


## Installation and Usage
The `cgroupv2_exporter` listens on HTTP port 9100 by default. See the `--help` output for more options.

## Collectors

Collectors are enabled by providing a `--collector.<name>` flag.
Collectors that are enabled by default can be disabled by providing a `--no-collector.<name>` flag.
To enable only some specific collector(s), use `--collector.disable-defaults --collector.<name> ...`.

### Enabled by default

#### Memory Collectors
Name     | Description
---------|-------------
memory.current | Current memory usage in bytes
memory.swap.current | Current swap usage in bytes
memory.high | Memory usage high threshold limit in bytes
memory.pressure | Memory pressure metrics (some, full, total, avg10, avg60, avg300)

#### CPU Collectors
Name     | Description
---------|-------------
cpu.pressure | CPU pressure metrics (some, full, total, avg10, avg60, avg300)
cpu.stat | CPU statistics (usage_usec, user_usec, system_usec, nr_periods, nr_throttled, throttled_usec)
cpuset.cpus | Number of CPUs in the cpuset
cpuset.cpus.effective | Number of effective CPUs in the cpuset
cpuset.mems | Number of memory nodes in the cpuset
cpuset.mems.effective | Number of effective memory nodes in the cpuset

#### I/O Collectors
Name     | Description
---------|-------------
io.pressure | I/O pressure metrics (some, full, total, avg10, avg60, avg300)
io.stat | I/O statistics per device (rbytes, wbytes, rios, wios, dbytes, dios)

### Disabled by default
Name     | Description
---------|-------------
memory.stat | Detailed memory statistics (anon, file, kernel_stack, slab, etc.) 

## Contributing
The code structure of cgroupv2_exporter is taken from [node_exporter](https://github.com/prometheus/node_exporter) and hence adding more collectors is also similar (see [collector](/collector) package).
The [parsers](/parsers) package provides parsers which can be used for converting for most of the cgroup files into p8s metrics.
