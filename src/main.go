package main

import (
	"context"
	"fmt"
	"os"
	"io"
	"encoding/json"
	"log"
	"regexp"
	"strings"
	"net/url"
	"net/http"
	"time"

	"github.com/mattn/go-mastodon"
)


// Define a struct for incoming program setting/configs
type ProgramConfig struct {
	LogBase             string `json:"logBase"` //The file path used to store logs.
	LogName             string `json:"logName"` //The file name used for program logs.
	ClientKey           string `json:"clientKey"` //The api ClientKey used to Access this service
	ClientSecret        string `json:"clientSecret"` //The api ClientSecret to Access this Service
	AccessToken         string `json:"accessToken"` //The api AccessToken to Access this service
	ServerURL           string `json:"serverURL"` //The Server address ex "https://mastodon.social
}

// Define a struct to partially match incoming JSON
type ResponseData struct {
	UserAcct	string `json:"acct"`
	DisplayName	string `json:"display_name"`
}

// Global Vars
var (
	programConfig	ProgramConfig //This object holds all program config options.
	startTime	time.Time //Holds the service's start time
	stdLogger	*log.Logger//New Logging updates should start here by removing all of these but one.
	acctRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
)

func main() {
	startTime := time.Now() //The program start time, used to output uptime.
	programArgs := os.Args[1:]
	configFile := programArgs[0]
	programConfig = loadProgramConfig(configFile)

	//Required Starting Args
	//ClientKey
	if (programConfig.ClientKey == "") {
		fmt.Print("ClientKey missing\n")
		os.Exit(1)
	}
	
	//ClientSecret
	if (programConfig.ClientSecret == "") {
		fmt.Print("ClientSecret missing\n")
		os.Exit(1)
	}
	
	//AccessToken
	if (programConfig.AccessToken == "") {
		fmt.Print("AcessToken missing\n")
		os.Exit(1)
	}
	
	//ServerURL
	if (programConfig.ServerURL == "") {
		fmt.Print("ServerURL missing\n")
		os.Exit(1)
	}

	//Logging Setup
	//Checks if the log directory exists if not it create it with permissions 0755
	if err := os.MkdirAll(programConfig.LogBase, 0755); err != nil {
		log.Fatalf("Fail to create directory: %v", err)
	}

	stdLogger = log.New(os.Stderr, "", log.LstdFlags | log.Lshortfile)//Initial stdLogger writes to /dev/null/
	stdLogger.Println("Standard Logger Enabled")
	stdLogger.Printf("Program Config File: %s\n", configFile)
	
	// Setup Mastodon Client w/ credentials from conf file.
	// Note: Mastodon's API Docs calls it "ClientKey" and the go-mastodon client calls it "ClientID"
	c := mastodon.NewClient(&mastodon.Config{
		Server:       programConfig.ServerURL,
		ClientID:     programConfig.ClientKey,
		ClientSecret: programConfig.ClientSecret,
		AccessToken:  programConfig.AccessToken,
	})

	// Save credentials for later usage if you wish to do so, config file, database, etc...
	fmt.Println("Client ID Used:", c.Config.ClientID)
	fmt.Println("Client Secret Used:", c.Config.ClientSecret)
	fmt.Println("Access Token Used:", c.Config.AccessToken)
	fmt.Println("ServerURL Used:", c.Config.Server)

	// Get Account Info from the mastodon client.
	acc, err := c.GetAccountCurrentUser(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	stdLogger.Println("Followers Count:", acc.FollowersCount)
	stdLogger.Println("Following Count:", acc.FollowingCount)
	
	// Get Lists
	getFollowers(acc, "")
	getFollowing(acc, "")

	elapsedTime := time.Now()
	fmt.Printf("Fetching FollowLists Finished. (Duration:%v)\n", elapsedTime.Sub(startTime))
	stdLogger.Printf("Fetching FollowLists Finished. (Duration:%v)\n", elapsedTime.Sub(startTime))
}

// parseMaxID extracts the "max_id" query parameter from the Link header.
// It looks for a link with rel="next" and returns the max_id value.
// Can be optimized
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

// Outputs all Following (Will be merged with Followers fuction)
// The API endpoint is: GET /api/v1/accounts/{id}/following?limit=80&max_id=nextID
// Will use an httpClient to fetch Following using the API and JSON
func getFollowing(acc *mastodon.Account, pageID string) {
	followingSet := make(map[string]struct{})
	pstring := ""
	
	for {
		//TODO: Update max and RateLimit Info using API
		//API Call
		httpClient := &http.Client{}
		apiURL := fmt.Sprintf("%s/api/v1/accounts/%s/following?limit=80&max_id=%s", programConfig.ServerURL, acc.ID, pstring)
		stdLogger.Printf("Get API URL:(%s)\n", apiURL)
		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			stdLogger.Printf("Error creating HTTP request: %+v", err)
			break
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			stdLogger.Printf("Error making HTTP request: %+v", err)
			break
		}
		defer resp.Body.Close()

		//Read the Response Body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			stdLogger.Printf("Error reading body: %+v", err)
			break
		}
	
		//Unmarshal the JSON data
		var data []ResponseData
		err = json.Unmarshal(body, &data)
		if err != nil {
			stdLogger.Printf("Error unmarshaling JSON: %+v", err)
			break
		}
	
		for position := range data {
			//Check its a properly formated User Account (UserName@Social.Example.org)
			//If no server info then the UserAcct is local.
			//Append local serverURL (programConf.serverURL) to the UserAcct string.
			u, _ := url.Parse(programConfig.ServerURL)
			if !acctRegex.MatchString(data[position].UserAcct) {
				data[position].UserAcct = data[position].UserAcct + "@" + u.Hostname()
			}
		
			//Save to map
			followingSet[data[position].UserAcct] = struct{}{}

			//Output to stdout
			fmt.Println("Following:", data[position].UserAcct)
		}

		//Read and output resp.Header information
		for key, values := range resp.Header {
			if key == "Link"{
				pstring, _ = parseMaxID(values[0])
				stdLogger.Printf("Parsed Max ID:%+v", pstring)
			}
			for _, value := range values {
				stdLogger.Printf("Other: %+v %+v", key, value)
			}
		}
		
		if pstring == "" {
			stdLogger.Printf("No more pages.")
			break
		}
	}

	return
}

// Outputs all Followers (Will be merged with Following function)
// The API endpoint is: GET /api/v1/accounts/{id}/follower?limit=80&max_id=nextID
// Will use an httpClient to fetch Followers using the API and JSON
func getFollowers(acc *mastodon.Account, pageID string) {
	followerSet := make(map[string]struct{})
	pstring := ""
	
	for {
		//TODO: Update max and RateLimit Info using API
		//API Call
		httpClient := &http.Client{}
		apiURL := fmt.Sprintf("%s/api/v1/accounts/%s/followers?limit=80&max_id=%s", programConfig.ServerURL, acc.ID, pstring)
		stdLogger.Printf("Get API URL:(%s)\n", apiURL)
		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			stdLogger.Printf("Error creating HTTP request: %+v", err)
			break
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			stdLogger.Printf("Error making HTTP request: %+v", err)
			break
		}
		defer resp.Body.Close()

		//Read the Response Body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			stdLogger.Printf("Error reading body: %+v", err)
			break
		}
	
		//Unmarshal the JSON data
		var data []ResponseData
		err = json.Unmarshal(body, &data)
		if err != nil {
			stdLogger.Printf("Error unmarshaling JSON: %+v", err)
			break
		}
	
		for position := range data {
			//Check its a properly formated User Account (UserName@Social.Example.org)
			//If no server info then the UserAcct is local.
			//Append local serverURL (programConf.serverURL) to the UserAcct string.
			u, _ := url.Parse(programConfig.ServerURL)
			if !acctRegex.MatchString(data[position].UserAcct) {
				data[position].UserAcct = data[position].UserAcct + "@" + u.Hostname()
			}
		
			//Save to map
			followerSet[data[position].UserAcct] = struct{}{}

			//Output to terminal
			fmt.Println("Follower:", data[position].UserAcct)
		}

		//Read and output resp.Header information
		for key, values := range resp.Header {
			if key == "Link"{
				pstring, _ = parseMaxID(values[0])
				stdLogger.Printf("Parsed Max ID: %+v", pstring)
			}
			for _, value := range values {
				stdLogger.Printf("Other: %+v %+v", key, value)
			}
		}
		
		if pstring == "" {
			stdLogger.Printf("No more pages.")
			break
		}
	}

	return
}

// loadProgramConfig: Might be a better way to do this.
func loadProgramConfig(file string) ProgramConfig {
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
