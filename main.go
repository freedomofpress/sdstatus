package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gocarina/gocsv"
	cli "github.com/jawher/mow.cli"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/proxy"
)

const (
	// proxyAddr points to local SOCKS proxy from Tor
	proxyAddr = "127.0.0.1:9050"
)

type SupportedLanguages []string

func (s SupportedLanguages) String() string {
	return strings.Join(s, " ")
}

// SecureDropMetadata stores the response from the SecureDrop metadata
// API endpoint
type SecureDropMetadata struct {
	Version            string             `json:"sd_version" csv:"sd_version"`
	Fingerprint        string             `json:"gpg_fpr" csv:"gpg_fpr"`
	SupportedLanguages SupportedLanguages `json:"supported_languages" csv:"supported_languages"`
}

// ScanResult stores the result of scanning a SecureDrop instance
type ScanResult struct {
	Title     string             `json:"title" csv:"title"`
	Url       string             `json:"url" csv:"url"`
	Available bool               `json:"available" csv:"available"`
	Error     string             `json:"error" csv:"error"`
	Metadata  SecureDropMetadata `json:"metadata" csv:"-"`
}

// checkStatus reads a SecureDrop site's metadata API endpoint
func checkStatus(wg *sync.WaitGroup, results chan ScanResult, updates chan string, client *http.Client, sd SecureDrop) {
	defer wg.Done()

	var result ScanResult
	result.Title = sd.Title

	updates <- fmt.Sprintf("Checking %s", sd.Title)

	metadataURL := fmt.Sprintf("http://%s/metadata", sd.OnionAddress)
	result.Url = metadataURL
	response, err := client.Get(metadataURL)
	if err != nil || response.StatusCode != 200 {
		var msg string
		if err != nil {
			msg = fmt.Sprintf("Error retrieving %s: %s", sd.Title, err)
		} else {
			msg = fmt.Sprintf("Error retrieving %s: status %s", sd.Title, http.StatusText(response.StatusCode))
		}
		log.Errorf(msg)

		result.Available = false
		result.Error = err.Error()
		results <- result
	} else {
		defer response.Body.Close()
		body, err := ioutil.ReadAll(response.Body)

		if err != nil {
			result.Available = false
			result.Error = err.Error()
			results <- result
		} else {
			var md SecureDropMetadata
			json.Unmarshal(body, &md)

			result.Metadata = md
			result.Available = true
			results <- result
		}
	}
	updates <- fmt.Sprintf("Finished checking %s", sd.Title)
}

// reportProgress prints messages from the updates channel
func reportProgress(updates chan string) {
	for update := range updates {
		log.Debugf(update)
	}
}

// runScan spawns crawlers of SecureDrop sites and collects their results
func runScan(client *http.Client, secureDrops map[string]SecureDrop, format string, output io.Writer) {
	resultList := make([]ScanResult, 0)

	results := make(chan ScanResult, len(secureDrops))
	updates := make(chan string, len(secureDrops))

	go reportProgress(updates)

	wg := sync.WaitGroup{}
	wg.Add(len(secureDrops))
	for _, sd := range secureDrops {
		go checkStatus(&wg, results, updates, client, sd)
	}

	// Now wait for all the results
	wg.Wait()
	close(results)

	for result := range results {
		resultList = append(resultList, result)
	}

	if format == "csv" {
		gocsv.Marshal(&resultList, output)
	} else {
		bits, err := json.MarshalIndent(resultList, "", "  ")
		if err == nil {
			io.WriteString(output, string(bits))
		} else {
			log.Errorf("Could not marshal results: %s\n", err)
		}
	}
	close(updates)
}

// readInputFile reads a CSV file of SecureDrop sites to scan,
// containing onionaddress,title on each line
func readInputFile(inputFile string) (secureDrops []SecureDrop, err error) {
	f, err := os.Open(inputFile)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	sds := []*SecureDrop{}
	if err = gocsv.UnmarshalFile(f, &sds); err != nil {
		log.Fatal(err)
	}
	for _, sd := range sds {
		secureDrops = append(secureDrops, *sd)
	}
	return
}

// makeClient makes a client for making HTTP requests over Tor
func makeClient(proxyAddress string, timeoutSeconds int) *http.Client {
	// create a SOCKS5 dialer
	dialer, err := proxy.SOCKS5("tcp", proxyAddress, nil, proxy.Direct)
	if err != nil {
		log.Fatalf("can't connect to the proxy: %s", err)
	}
	// setup the http client
	httpTransport := &http.Transport{}
	// Add the dialer
	httpTransport.Dial = dialer.Dial

	timeout := time.Duration(timeoutSeconds) * time.Second
	return &http.Client{Timeout: timeout, Transport: httpTransport}
}

func scan(cmd *cli.Cmd) {
	cmd.Spec = "[OPTIONS] [ONION_URL...]"
	var (
		directory  = cmd.BoolOpt("d directory", false, "Read sites to scan from the securedrop.org directory ")
		format     = cmd.StringOpt("f format", "json", "Specify output format: \"csv\" or \"json\"")
		inputFile  = cmd.StringOpt("i inputFile", "", "Read sites to scan from a CSV file having \"address,title\" on each line")
		outputFile = cmd.StringOpt("o outputFile", "", "Direct output to the named file instead of the terminal")
		timeout    = cmd.IntOpt("t timeout", 60, "Maximum time in seconds to wait for a response from a SecureDrop site")
		verbose    = cmd.BoolOpt("v verbose", false, "Be verbose when filtering words")
		onion_urls = cmd.StringsArg("ONION_URL", []string{}, "Onion URLs to scan")
	)

	cmd.Action = func() {
		if *verbose {
			log.SetLevel(log.DebugLevel)
		}

		if *format != "csv" && *format != "json" {
			log.Fatalf("Output format may only be JSON or CSV.")
		}

		if *inputFile == "" && *directory == false && len(*onion_urls) == 0 {
			log.Fatalf("Please supply sites to scan on the command line or with --directory or --inputFile.")
		}

		secureDrops := make(map[string]SecureDrop)

		for _, onion_url := range *onion_urls {
			log.Printf("Will scan %s", onion_url)
			secureDrops[onion_url] = SecureDrop{OnionAddress: onion_url, Title: onion_url}
		}

		if *inputFile != "" {
			log.Printf("Reading SecureDrops to scan from %s", *inputFile)
			sdd, err := readInputFile(*inputFile)
			if err != nil {
				log.Fatalf("Could not read Onion addresses from %s: %v", inputFile, err)
			}
			for _, sd := range sdd {
				secureDrops[sd.OnionAddress] = sd
			}
		}

		client := makeClient(proxyAddr, *timeout)

		if *directory {
			sddir, err := GetDirectory(client)
			if err != nil {
				log.Fatalf("Could not read Onion addresses from the SecureDrop directory: %v", err)
			}
			for _, sd := range sddir {
				secureDrops[sd.OnionAddress] = sd
			}
		}

		output := bufio.NewWriter(os.Stdout)
		if *outputFile != "" {
			f, err := os.Create(*outputFile)
			defer f.Close()
			if err != nil {
				log.Fatalf("Could not open output file %s: %s", outputFile, err)
			}
			output = bufio.NewWriter(f)
		}
		runScan(client, secureDrops, *format, output)
		output.Flush()
	}
}

func l10n(cmd *cli.Cmd) {
	cmd.Spec = "INPUTFILE"
	var (
		inputFile = cmd.StringArg("INPUTFILE", "", "The JSON output of a previous \"scan\"")
	)
	cmd.Action = func() {
		if *inputFile == "" {
			log.Fatalf("Please supply the JSON output file from a previous scan.")
		}

		resultJSON, err := ioutil.ReadFile(*inputFile)
		if err != nil {
			log.Fatal(err)
		}

		scanResults := make([]ScanResult, 0)
		if err = json.Unmarshal(resultJSON, &scanResults); err != nil {
			log.Fatal(err)
		}

		localeSites := make(map[string][]string)
		for _, result := range scanResults {
			for _, locale := range result.Metadata.SupportedLanguages {
				localeSites[locale] = append(localeSites[locale], result.Title)
			}
		}

		var locales []string
		for locale, _ := range localeSites {
			locales = append(locales, locale)
		}
		sort.Strings(locales)

		for _, locale := range locales {
			sites := localeSites[locale]
			sort.Strings(sites)
			fmt.Printf("%v (%d):\n  %v\n\n", locale, len(sites), strings.Join(sites, "\n  "))
		}
	}
}

func main() {
	log.SetFormatter(&log.TextFormatter{
		DisableLevelTruncation: true,
		DisableTimestamp:       true,
	})

	app := cli.App("sdstatus", "Reports metadata about SecureDrop sites")
	app.Command("scan", "Retrieve metadata from SecureDrop sites", scan)
	app.Command("l10n", "Reports localization metrics from scanned metadata", l10n)
	app.Run(os.Args)
}
