package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"golang.org/x/net/proxy"
)

const (
	PROXY_ADDR = "127.0.0.1:9050"
	URL        = "https://httpbin.org/get"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
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
	httpClient := &http.Client{Transport: httpTransport}
	// Add the dialer
	httpTransport.Dial = dialer.Dial
	// Create the request
	req, err := http.NewRequest("GET", URL, nil)
	check(err)

	resp, err := httpClient.Do(req)
	check(err)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	check(err)
	fmt.Println(string(body))
}
