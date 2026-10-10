package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/VerS7/mediadump/parse"
)

var (
	InputFilePath  = flag.String("input", "links.txt", "file with URLs to process")
	OutputFilePath = flag.String("output", "links.json", "processed json")

	ParseOpenGraph = flag.Bool("parse-og", false, "parse OpenGraph data from URLs")
	HttpWorkers    = flag.Int("workers", 5, "parallel workers for parsing http")
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Mediadump Links Parser\n")
		flag.PrintDefaults()
	}

	flag.Parse()
}

func main() {
	var firstDate, lastDate int64
	var count int
	var parsedLinks []parse.LinkEntry

	firstDate = math.MaxInt64

	inputFile, err := os.OpenFile(*InputFilePath, os.O_RDONLY, 0666)
	if err != nil {
		panic(err)
	}
	outputFile, err := os.OpenFile(*OutputFilePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	if err != nil {
		panic(err)
	}

	domains := make(map[string]int)

	scanner := bufio.NewScanner(inputFile)
	if err := scanner.Err(); err != nil {
		panic(err)
	}

	fmt.Println("Starting to process URLs...")

	for scanner.Scan() {
		rawLine := scanner.Text()

		lk, err := parse.ParseRawLine(rawLine)
		if err != nil {
			fmt.Println(fmt.Errorf("error parsing line '%s': %s", rawLine, err))
			continue
		}

		count += 1
		parsedLinks = append(parsedLinks, lk)

		if lk.Unix < firstDate {
			firstDate = lk.Unix
		}
		if lk.Unix > lastDate {
			lastDate = lk.Unix
		}

		domains[lk.Domain] += 1
	}

	if *ParseOpenGraph {
		fmt.Println("Begin OpenGraph parsing...")
		pool := parse.NewHttpParsePool(false)

		pool.Swarm(*HttpWorkers)
		defer pool.Close()

		jobs := make([]parse.HttpParseJob, 0, len(parsedLinks))
		for i := 0; i < len(parsedLinks); i += 1 {
			jobs = append(jobs, parse.HttpParseJob{
				URL: parsedLinks[i].Url,
				ID:  i,
			})
		}

		pool.Feed(jobs)

		processed := 0

		for result := range pool.Results() {
			processed += 1

			if result.Error != nil {
				fmt.Println(result.Error)
				continue
			}

			fmt.Printf("%d/%d processed...\n", processed, len(parsedLinks))

			parsedLinks[result.Job.ID].OGData = result.OGData
		}
	}

	domainEntries := make([]parse.DomainEntry, 0, len(domains))

	for d, c := range domains {
		domainEntries = append(domainEntries, parse.DomainEntry{Domain: d, Count: c})
	}

	linksInfo := parse.ParsedLinksInfo{
		Count:     count,
		FirstDate: time.Unix(firstDate, 0).String(),
		LastDate:  time.Unix(lastDate, 0).String(),
		Links:     parsedLinks,
		Domains:   domainEntries,
	}

	jsonData, err := json.Marshal(linksInfo)
	if err != nil {
		panic(err)
	}

	_, err = outputFile.Write(jsonData)
	if err != nil {
		panic(err)
	}

	fmt.Println("Done!")
}
