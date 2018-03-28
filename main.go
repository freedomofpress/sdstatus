package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"golang.org/x/net/proxy"
)

const (
	// proxyAddr points to local SOCKS proxy from Tor
	proxyAddr = "127.0.0.1:9050"
)

// Information is used in channels
type Information interface {
	msg() string
}

// SDMetadata stores JSON metadata from SD instances
type SDMetadata struct {
	Version     string `json:"sd_version"`
	Fingerprint string `json:"gpg_fpr"`
}

// SDInfo stores metadata and Onion URL
type SDInfo struct {
	Info      SDMetadata
	Url       string
	Available bool
}

func (sd SDInfo) msg() string {
	msgstr := fmt.Sprintf("%s,%s,%s", sd.Url, sd.Info.Version, sd.Info.Fingerprint)
	return msgstr
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func checkStatus(ch chan Information, client *http.Client, url string) {
	var result SDInfo
	result.Url = url

	metadataURL := fmt.Sprintf("http://%s/metadata", url)
	// Create the request
	req, err := http.NewRequest("GET", metadataURL, nil)
	if err != nil {
		result.Available = false
		ch <- result
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		result.Available = false
		ch <- result
		return
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		result.Available = false
		ch <- result
		return
	}

	var info SDMetadata
	json.Unmarshal(body, &info)

	result.Info = info
	result.Available = true
	ch <- result
}

func main() {
	i := 0

	results := make([]SDInfo, 0)
	// create a SOCKS5 dialer
	dialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
	if err != nil {
		fmt.Fprintln(os.Stderr, "can't connect to the proxy:", err)
		os.Exit(1)
	}
	// setup the http client
	httpTransport := &http.Transport{}
	c := &http.Client{Transport: httpTransport}
	// Add the dialer
	httpTransport.Dial = dialer.Dial

	ch := make(chan Information)

	// Now let us find the onion addresses
	data, err := ioutil.ReadFile("sdonion.txt")
	check(err)
	lines := strings.Split(string(data), "\n")

	for _, v := range lines {
		url := strings.TrimSpace(v)

		if url != "" {
			go checkStatus(ch, c, v)
			i = i + 1
		}

	}

	for {
		result := <-ch
		if result != nil {

			//fmt.Println(result.msg())
			results = append(results, result.(SDInfo))
			i = i - 1
		}
		if i == 0 {
			break
		}
	}

	bits, err := json.MarshalIndent(results, "", "\t")
	if err == nil {
		fmt.Println(string(bits))
	}

}
