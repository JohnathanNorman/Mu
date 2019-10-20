package main

import (
	"crypto/rand"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"time"
)

var conf Config

// RunResult provides the results for each command run
type RunResult struct {
	stdOut    string
	stdErr    string
	returnVal int
	timedOut  bool
	pid       int
}

// Config object for json
type Config struct {
	Strategy           string   `json:"strategy"`
	Consumer           string   `json:"consumer"`
	ConsumerArgs       string   `json:"consumerArgs"`
	Producer           string   `json:"producer"`
	ProducerArgs       string   `json:"producerArgs"`
	CrashDir           string   `json:"crashDir"`
	DebugMode          bool     `json:"debugMode"`
	Coverage           bool     `json:"coverage"`
	Threads            int      `json:"threads"`
	ConsumerTimeout    int      `json:"consumerTimeout"`
	ProducerTimeout    int      `json:"producerTimeout"`
	InterestingStrings []string `json:"interestingStrings"`
	MaxRunTime         int      `json:"maxRunTime"`
}

//Job details worker info
type Job struct {
	id         int
	iterations int
	crashes    int
	intStrings int
}

var itercount int
var ticker *time.Ticker
var wg sync.WaitGroup
var jobchan = make(chan Job, 100)
var covchan = make(chan []int64, 100)
var cov = Int64Set{}

func main() {

	itercount = 50

	ticker = time.NewTicker(1 * time.Minute)

	if os.Args[1] == "" {
		log.Fatalln("Please provide config file")
	}

	jsonFile, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Println(err)
	}

	byteValue, _ := ioutil.ReadAll(jsonFile)
	json.Unmarshal(byteValue, &conf)
	defer jsonFile.Close()

	for w := 1; w <= conf.Threads; w++ {
		wg.Add(1)
		go worker(w)
	}
	fmt.Println(conf.Threads, "workers started")

	wg.Add(1)
	go loop()
	wg.Wait()

}

func worker(id int) {
	defer wg.Done()
	w := NewWorker(conf)
	w.start(id)
}

// if strategy name defined name file accordingly.
func getStatsFN() string {
	if len(conf.Strategy) > 0 {
		return fmt.Sprintf("%s_fuzzstats.csv", conf.Strategy)
	}
	return "fuzzstats.csv"
}

func loop() {
	defer wg.Done()
	var totalIters int
	var totalCrashes int
	var totalInter int

	startTime := time.Now()
	statfh, ferr := os.Create(getStatsFN())
	if ferr != nil {
		log.Fatalln("error trying to create stats file", ferr)
	}

	// create stats csv and write header
	statcsv := csv.NewWriter(statfh)
	if conf.Coverage {
		statcsv.Write([]string{"Runtime(seconds)", "Iterations", "Iterations/Second", "Crashes", "Interesting", "Coverage"})
	} else {
		statcsv.Write([]string{"Runtime(seconds)", "Iterations", "Iterations/Second", "Crashes", "Interesting"})
	}

	for {
		select {
		case jobInfo := <-jobchan:
			totalIters = totalIters + jobInfo.iterations
			totalCrashes = totalCrashes + jobInfo.crashes
			totalInter = totalInter + jobInfo.intStrings
		case <-ticker.C:
			//every minute print our stats
			runtime := time.Since(startTime).Round(time.Millisecond)
			itersec := float64(totalIters) / runtime.Seconds()
			if conf.Coverage {
				fmt.Printf("runtime:%s iterations:%d iter/sec:%.3f crashes:%d interesting:%d coverage:%d\n", fmtDuration(runtime), totalIters, itersec, totalCrashes, totalInter, cov.Size())
			} else {
				fmt.Printf("runtime:%s iterations:%d iter/sec:%.3f crashes:%d interesting:%d\n", fmtDuration(runtime), totalIters, itersec, totalCrashes, totalInter)
			}
			// also log our data
			statout := []string{fmt.Sprintf("%.3f", runtime.Seconds()), fmt.Sprintf("%d", totalIters), fmt.Sprintf("%.3f", itersec),
				fmt.Sprintf("%d", totalCrashes), fmt.Sprintf("%d", totalInter), fmt.Sprintf("%d", cov.Size())}
			statcsv.Write(statout)
			statcsv.Flush()

			if exceedRunTime(startTime) {
				fmt.Println("Runtime exceeded , exiting...")
				os.Exit(0)
			}

		case covdata := <-covchan:
			for _, coverage := range covdata {
				cov.Add(coverage)
			}
		}
	}
}

// check if we exceed desired runtime. 0 = run forever
func exceedRunTime(t time.Time) bool {
	if conf.MaxRunTime == 0 {
		return false
	}

	runtime := time.Since(t).Round(time.Minute)
	if runtime.Minutes() >= float64(conf.MaxRunTime) {
		return true
	}
	return false
}

func uuid() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		log.Fatal(err)
	}
	uuid := fmt.Sprintf("%x-%x-%x-%x-%x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
	return uuid
}

func copy(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer destination.Close()
	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}

func fmtDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	return fmt.Sprintf("%02d:%02d", h, m)
}
