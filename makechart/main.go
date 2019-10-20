package main

import (
	"encoding/csv"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/wcharczuk/go-chart"
)

func main() {

	argsWithoutProg := os.Args[1:]
	if len(argsWithoutProg) < 2 {
		log.Fatalln("provide 2 filenames")
	}

	runtime1, coverage1, series1 := getChartData(argsWithoutProg[0])
	runtime2, coverage2, series2 := getChartData(argsWithoutProg[1])
	graph := chart.Chart{
		Background: chart.Style{
			Padding: chart.Box{
				Top:  20,
				Left: 260,
			},
		},
		Series: []chart.Series{

			chart.ContinuousSeries{
				Name:    series1,
				XValues: runtime1,
				YValues: coverage1,
			},
			chart.ContinuousSeries{
				Name:    series2,
				XValues: runtime2,
				YValues: coverage2,
			},
		},
	}

	graph.Elements = []chart.Renderable{
		chart.Legend(&graph),
	}

	f, _ := os.Create("chart.png")
	defer f.Close()
	graph.Render(chart.PNG, f)
}

func getChartData(csvfile string) ([]float64, []float64, string) {

	//"haiyocpp_d8_fuzzstats.csv"
	series := strings.Split(csvfile, "_fuzzstats")

	csvFile, _ := os.Open(csvfile)
	r := csv.NewReader(csvFile)

	var coverage []float64
	var runtime []float64
	var iteration int

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		if iteration > 0 {
			r, rerr := strconv.ParseFloat(record[0], 64)
			if rerr != nil {
				log.Fatalln("Can't parse CSV", rerr)
			}
			runtime = append(runtime, r)

			c, cerr := strconv.ParseFloat(record[5], 64)
			if cerr != nil {
				log.Fatalln("Can't parse file", cerr)
			}
			coverage = append(coverage, c)
		}
		iteration = iteration + 1
	}

	return runtime, coverage, series[0]

}
