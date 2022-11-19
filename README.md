# table-diff

table-diff is a powerful but straightforward command line tool to find differences between two tables.

watch the demo:
[![asciicast](https://asciinema.org/a/7STVpYPdgX5K1plezWVodDiE8.svg)](https://asciinema.org/a/7STVpYPdgX5K1plezWVodDiE8)

# Installation

you can install this tool via github
```
go install github.com/faycheng/table-diff@latest
```

of course, you can also download the released binary directly

# Quick Start

1, create your own diff configuration file which contains all necessary information for running a table-diff task, then save it as `diff-table.toml`

```
[Left]
    DSN = "root:password@tcp(127.0.0.1:13306)/table_diff"
    Table = "table_diff_test"

[Right]
    DSN = "root:password@tcp(127.0.0.1:13307)/table_diff"
    Table = "table_diff_test"
```

2, launch the table-diff tool with the created configuration file
```
table-diff --conf ./diff-table.toml
```

# Configurations

```
# the column name of primary key in the table, default is `id`
PrimaryKey = "id"
# the column names for comparing differences, default are all columns in the table
DiffColumns = ["id", "col01", "col02"]
[Left]
    # the database source configuration
    DSN = "root:password@tcp(127.0.0.1:13306)/table_diff"
    # the table name for comparing differences
    Table = "table_diff_test"
[Right]
    # the database source configuration
    DSN = "root:password@tcp(127.0.0.1:13307)/table_diff"
    # the table name for comparing differences
    Table = "table_diff_test"
[Segment]
    # the min value of primary key in the comparing range, default is 0
    From = 0
    # the max value of primary key in the comparing range, default is the current max row id in the table
    To = 10000
    # the batch size in every segment, default is 1000
    Step = 10
[Concurrency]
    # the size of the concurrently running worker, default is 1
    Worker = 4
[Output]
    # the file path for saving the detected differences on the disk, default is `/tmp/table-diff-{uuid}.diff`
    File = "/tmp/table-diff.diff"
```

# Performance Benchmark

benchmark environment:
- cpu: 40 cores, memory: 90 GB
- database: MySQL 8.0
- rows: 10,000,000 rows, 28 columns per table

### benchmark report

case 1: segStep=1000, worker=16
how long the comparing task takes to run?
0m14.038s

case 2: segStep=2000, worker=16
how long the comparing task takes to run?
0m13.858s

case 3: segStep=5000, worker=16
how long the comparing task takes to run?
0m13.935s

case 4: segStep=2000, worker=8
how long the comparing task takes to run?
0m14.161s

case 5: segStep=2000, worker=32
how long the comparing task takes to run?
0m16.262s

# Usage Limitation

- only support two types of relation databases: MySQL and MariaDB
- the primary key of these two tables must be `int`
- the primary key of these two tables must be auto-incremented


# Design

this project is inspired by the `https://github.com/datafold/data-diff`.
if you want to know the technical explanation in detail, you can visit: `https://docs.datafold.com/os_diff/technical_explanation`.