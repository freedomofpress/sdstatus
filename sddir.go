package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"
	log "github.com/sirupsen/logrus"
)

const (
	SecureDropDirectoryURL = "https://securedrop.org/api/v1/directory/"
)

type OrganizationLogo struct {
	Height int `json:"height"`
	Width int `json:"width"`
	URL string `json:"url"`
}

type SecureDrop struct {
	Title string `json:"title" csv:"title"`
	Slug string `json:"slug" csv:"slug"`
	DirectoryURL string `json:"directory_url" csv:"directory_url"`
	FirstPublishedAt time.Time `json:"first_published_at" csv:"first_published_at"`
	LandingPageURL string `json:"landing_page_url" csv:"landing_page_url"`
	OnionAddress string `json:"onion_address" csv:"onion_address"`
	OrganizationLogo OrganizationLogo `json:"organization_logo" csv:"organization_logo"`
	OrganizationDescription string `json:"organization_description" csv:"organization_description"`
	Languages []string `json:"languages" csv:"languages"`
	Topics []string `json:"topics" csv:"topics"`
	Countries []string `json:"countries" csv:"countries"`
}

func GetDirectory(client *http.Client) (sd []SecureDrop, err error) {
	log.Printf("Reading SecureDrops to scan from %s", SecureDropDirectoryURL)
	request, err := http.NewRequest("GET", SecureDropDirectoryURL, nil)
	if err != nil {
		return
	}

	response, err := client.Do(request)
	if err != nil {
		return
	}

	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return
	}

	err = json.Unmarshal(body, &sd)
	if err != nil {
		return
	}

	return sd, nil
}
