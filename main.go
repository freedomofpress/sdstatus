package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"golang.org/x/net/proxy"
)

const (
	PROXY_ADDR = "127.0.0.1:9050"
	URL        = "http://p6qn2dviaa53hre5.onion/metadata"
)

type Information interface {
	msg() string
}

type SDJson struct {
	Version     string `json:"sd_version"`
	Fingerprint string `json:"gpg_fpr"`
}

type SDInfo struct {
	Info   SDJson
	Url    string
	Status bool
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
	// Create the request
	req, err := http.NewRequest("GET", URL, nil)
	if err != nil {
		result.Status = false
		ch <- result
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		result.Status = false
		ch <- result
		return
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		result.Status = false
		ch <- result
		return
	}

	var info SDJson
	json.Unmarshal(body, &info)

	result.Info = info
	ch <- result
}

func main() {
	// create a SOCKS5 dialer
	dialer, err := proxy.SOCKS5("tcp", PROXY_ADDR, nil, proxy.Direct)
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

	go checkStatus(ch, c, URL)

	for {
		result := <-ch
		if result != nil {
			fmt.Println(result.msg())
			break
		}
	}

}
