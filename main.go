package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/k0kubun/pp"
	"github.com/urfave/cli"
	"golang.org/x/net/proxy"
)

const (
	// proxyAddr points to local SOCKS proxy from Tor
	proxyAddr = "127.0.0.1:9050"
)

// SDMetadata stores the information obtained from a given SecureDrop
// instance's /metadata endpoint, a JSON API with platform info.
type SDMetadata struct {
	Version     string `json:"sd_version"`
	Platform    string `json:"server_os"`
	Fingerprint string `json:"gpg_fpr"`
	V2SourceURL string `json:"v2_source_url"`
	V3SourceURL string `json:"v3_source_url"`
}

// SDInstance stores metadata and Onion URL
type SDInstance struct {
	Metadata       SDMetadata
	Url            string `json:"onion_address"`
	Title          string `json:"title"`
	Available      bool
	DirectoryUrl   string `json:"directory_url"`
	LandingPageUrl string `json:"landing_page_url"`
}

func checkStatus(client *http.Client, sd SDInstance) SDInstance {
	metadataURL := fmt.Sprintf("http://%s/metadata", sd.Url)

	sd.Available = false
	// Create the request
	req, err := http.NewRequest("GET", metadataURL, nil)
	if err != nil {
		return sd
	}

	resp, err := client.Do(req)
	if err != nil {
		return sd
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return sd
	}

	var info SDMetadata
	json_err := json.Unmarshal(body, &info)
	if json_err != nil {
		log.Fatal(json_err)
	}

	sd.Metadata = info
	sd.Available = true
	return sd
}

func getSecureDropDirectory() []SDInstance {
	response, err := http.Get("https://securedrop.org/api/v1/directory/")
	if err != nil {
		log.Fatal(fmt.Sprintf("%s", err))
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatal(fmt.Sprintf("%s", err))
	}
	var directoryResults []SDInstance
	json_err := json.Unmarshal(body, &directoryResults)
	if json_err != nil {
		log.Fatal(json_err)
	}
	return directoryResults
}

func runScan(ch chan SDInstance, sdInstances []SDInstance, format string) {
	var wg sync.WaitGroup
	// create a SOCKS5 dialer
	dialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
	if err != nil {
		log.Fatal("can't connect to the proxy:", err)
	}
	// setup the http client
	httpTransport := &http.Transport{}
	client := &http.Client{Transport: httpTransport}
	// Add the dialer
	httpTransport.Dial = dialer.Dial

	// For each address we are creating a goroutine
	for _, i := range sdInstances {
		wg.Add(1)
		go func(i SDInstance) {
			defer wg.Done()
			r := checkStatus(client, i)
			ch <- r
		}(i)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()
}

func displayInstance(sd SDInstance, format string) {
	if format == "pp" {
		pp.Print(sd)
	} else if format == "csv" {
		s := fmt.Sprintf("%t,%s,%s,%s", sd.Available, sd.Metadata.Version, sd.Title, sd.Url)
		fmt.Println(s)
	} else if format == "json" {
		sd_j, err := json.Marshal(sd)
		if err != nil {
			log.Println(err)
		}
		fmt.Printf("%s\n", sd_j)
	}
}

func createApp() *cli.App {
	app := cli.NewApp()
	var format string
	app.EnableBashCompletion = true
	app.Name = "sdstatus"
	app.Version = "0.1.0"
	app.Usage = "To scan SecureDrop instances"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "format",
			Usage:       "Output scan results in `FORMAT`: json, pp, csv",
			Value:       "csv",
			Destination: &format,
		},
	}
	app.Action = func(c *cli.Context) error {
		// onion_urls := c.Args()
		// sd_instances := make([]SDInstance, 0)
		sd_instances := getSecureDropDirectory()
		ch := make(chan SDInstance)
		runScan(ch, sd_instances, format)
		for {
			x, ok := <-ch
			if ok == false {
				break
			}
			displayInstance(x, format)
		}
		return nil
	}

	return app
}

func main() {
	app := createApp()
	if err := app.Run(os.Args); err != nil {
		panic(err)
	}
}
