
# Mu


## Overview

Mu is a simple fuzzing harness written in Go that can be used to measure the effectiveness of different fuzzing stratgies. Users can run a fuzzer for a specified period of time, measure code coverage and easily compare the results against previous runs. Fuzzer performance is logged to a simple CSV which can be analyzed using a variety of tools such as Excel, Python etc. A simple cmd line utility is included to chart performance as well. 

The harness is multi-threaded and can be run without code coverage for general purpose fuzzing. Everything is cross platform and should work on Linux/Windows.  


## Building
To run simply build by running "go build" from the directory.
The program takes a single command line argument which is the configuration file in a json format. Examples of this are in the "conf" directory

to build the chart utility 
```
go get -u github.com/wcharczuk/go-chart
cd makechart
go build
```

code coverage data requires [Sanitizer Coverage](https://clang.llvm.org/docs/SanitizerCoverage.html) so your target (consumer) should be compiled with compiled with -fsanitize=address and -fsanitize-coverage=trace-pc-guard. 

## Example
lets say we want to fuzz d8 (a frontend to [V8](https://v8.dev/)) and we want to compare our fuzzer (haiyo) to [Domato](https://github.com/googleprojectzero/domato)

haiyocpp_d8.json

```json
{
    "strategy": "haiyocpp_d8",
    "consumer": "/home/user/src/chromium/src/out/Asan/d8",
    "consumerArgs": "--expose-gc",
    "producer": "/home/user/src/haiyo_cpp/haiyo",
    "producerArgs": "<outdir> <count>",
    "crashDir": "/home/user/crashes",
    "coverage": true,
    "debugMode": false,
    "threads": 20,
    "consumerTimeout": 10,
    "producerTimeout": 30,
    "maxRunTime": 10,
    "interestingStrings": ["stack", "crash"]
}
```


domato_d8.json
```json
{
    "strategy": "domato_d8",
    "consumer": "/home/user/src/chromium/src/out/Asan/d8",
    "consumerArgs": "--expose-gc",
    "producer": "python",
    "producerArgs": "/home/user/src/domato/jscript/generator.py --output_dir <outdir> --no_of_files <count> ",
    "crashDir": "/home/user/crashes",
    "coverage": true,
    "debugMode": false,
    "threads": 20,
    "consumerTimeout": 10,
    "producerTimeout": 30,
    "maxRunTime": 10,
    "interestingStrings": ["stack", "crash"]
}
```

Run the harness for both configs

```
./Mu conf/haiyocpp_d8.json && ./Mu domato_d8.json

20 workers started
runtime:00:01 iterations:4825 iter/sec:80.417 crashes:0 interesting:0 coverage:41330
runtime:00:02 iterations:9625 iter/sec:80.208 crashes:0 interesting:0 coverage:42734
runtime:00:03 iterations:13450 iter/sec:74.722 crashes:0 interesting:0 coverage:43409
runtime:00:04 iterations:17000 iter/sec:70.833 crashes:0 interesting:0 coverage:43983
runtime:00:05 iterations:21250 iter/sec:70.833 crashes:0 interesting:0 coverage:44321
runtime:00:06 iterations:25200 iter/sec:70.000 crashes:0 interesting:0 coverage:44751
runtime:00:07 iterations:29475 iter/sec:70.178 crashes:0 interesting:0 coverage:45307
runtime:00:08 iterations:33350 iter/sec:69.479 crashes:0 interesting:0 coverage:45523
runtime:00:09 iterations:37200 iter/sec:68.889 crashes:0 interesting:0 coverage:45810
runtime:00:10 iterations:41625 iter/sec:69.375 crashes:0 interesting:0 coverage:46088
Runtime exceeded , exiting...

```

This will run the harness for 10 minutes and produce \<strategy>_fuzzstats.csv. Any crashes identified will be logged with the relevant testcase. The harness will also check stderr and stdout for any interesting strings and log those test cases as well. 

You can compare coverage with ..

```
./makechart domato_d8_fuzzstats.csv haiyocpp_d8_fuzzstats.csv
```
And as you can see below our difference in coverage

<p align="center">
  <img src="https://raw.githubusercontent.com/JohnathanNorman/Mu/master/makechart/output.png" alt="Chart"/>
</p>


## Contributions
Sure why not?
