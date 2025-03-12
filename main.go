package main

import (
	"context"
	"fmt"
	"os"
	"encoding/json"
	"log"
	"strings"
	"net/url"
	"net/http"

	"github.com/mattn/go-mastodon"
)

type ProgramConfig struct {
	LogBase             string `json:"logBase"` //The file path used to store logs.
	LogName             string `json:"logName"` //The file name used for program logs.
	LogLevel	    string `json:"logLevel"`//The program's log level. (Currently Only 1 Level)
	DataBase	    string `json:"database"`//The file path used to store data.
	DataName	    string `json:"dataname"`//The file name used for program data.
	Port		    int    `json:"port"` //The port used for the ProductIngest service.
	UpdateIntervel      int    `json:"updateIntervel"` //The amount of time inbetween info updates, min 60 seconds.
	GlobalMarkupPercent int    `json:"globalMarkupPercent"` //The markup percent for all products regardless of supplier.
	GlobalThrottle      int    `json:"globalThrottle"` //The throttle percent used for all incoming inventory.
	APIKey		    string `json:"apiKey"` //The api key used to access this service.
}

func main() {
	// Replace with your Mastodon credentials
	clientID := ""
	clientSecret := ""
	accessToken := ""
	serverURL := "https://mastodon.social" // Change based on your instance

	c := mastodon.NewClient(&mastodon.Config{
		Server:       serverURL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		AccessToken:  accessToken,
	})

	// Save credentials for later usage if you wish to do so, config file, database, etc...
	fmt.Println("ClientID:", c.Config.ClientID)
	fmt.Println("ClientSecret:", c.Config.ClientSecret)
	fmt.Println("Access Token:", c.Config.AccessToken)

	// Lookup and get account id
	acc, err := c.AccountLookup(context.Background(), "MeaningfulBits")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(acc)
		
	fmt.Println(getFollowersPage(c, ""))
}

// parseMaxID extracts the "max_id" query parameter from the Link header.
// It looks for a link with rel="next" and returns the max_id value.
func parseMaxID(linkHeader string) (string, error) {
	// Split multiple links separated by commas.
	links := strings.Split(linkHeader, ",")
	for _, link := range links {
		// Each link is expected in the form:
		// <URL>; rel="next"
		parts := strings.Split(link, ";")
		if len(parts) < 2 {
			continue
		}
		// Trim the angle brackets and whitespace from the URL.
		urlPart := strings.Trim(parts[0], " <>")
		// Check the relation type.
		relPart := strings.TrimSpace(parts[1])
		if relPart == `rel="next"` {
			// Parse the URL.
			parsedUrl, err := url.Parse(urlPart)
			if err != nil {
				return "", fmt.Errorf("error parsing URL: %w", err)
			}
			// Extract the "max_id" query parameter.
			maxID := strings.TrimSpace(parsedUrl.Query().Get("max_id"))
			if maxID != "" {
				return maxID, nil
			}
		}
	}
	return "", fmt.Errorf("max_id not found in Link header")	
}

func getFollowersPage(c *mastodon.Client, pageID string)(string){
	pager := mastodon.Pagination{
		Limit: 80,
	}
	
	// Lookup and get account id
	acc, err := c.AccountLookup(context.Background(), "MeaningfulBits")
	if err != nil {
		log.Fatal(err)
	}
	
	// Get the the usernames of followers using the account ID
	followers, err := c.GetAccountFollowers(context.Background(), acc.ID, &pager)
	if err != nil {
		log.Fatal(err)
	}
		
	for _, f := range followers {
		fmt.Println(f.Username)
	}
	
	//Update Max_ID and Ratelimit Info using API
	//Fetch the Header (Manually call the same Mastodon API endpoint to capture response headers.)
	// The API endpoint is: GET /api/v1/accounts/{id}/follower?limit=80&max_id=nextID
	url := fmt.Sprintf("https://mastodon.social/api/v1/accounts/%s/followers?limit=80&max_id=%s", acc.ID, pageID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalf("Error creating HTTP request: %v", err)
	}
		
	// Set the Authorization header using your access token.
	//Broken
	req.Header.Set("Authorization", "Bearer xxxxxx")

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Fatalf("Error making HTTP request: %v", err)
	}
	defer resp.Body.Close()
		
	fmt.Println(resp.Header)
		
	for key, values := range resp.Header {
		if key == "Link"{
			pstring, err := parseMaxID(values[0])
			if err != nil {
				log.Fatalf("Error with parse MaxID: %v", err)
			}
			fmt.Println("Parsed Max ID:", pstring)
			pageID = pstring
		}
		for _, value := range values {
			fmt.Println("Other: ", key, value)
		}
	}
		
	fmt.Println("nextID:", pageID)
	fmt.Println("Follower List Length:", len(followers))
	
	return "Broken"
}

func LoadProgramConfig(file string) ProgramConfig {
	configFile, err := os.Open(file)
		defer configFile.Close()
	if err != nil {
		fmt.Println(err.Error())
	}
	decoder := json.NewDecoder(configFile)
	var programConfig ProgramConfig
	dErr := decoder.Decode(&programConfig)
	if dErr != nil{
		fmt.Printf("Error Decoding: %s\n", dErr)
	}
	return programConfig
}
