package main

import (
	"context"
	"fmt"
	"os"
	"io"
	"encoding/json"
	"log"
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
	DataBase	    string `json:"dataBase"`//The file path used to store data.
	ClientID            string `json:"clientID"` //The api ClientID used to Access this service
	ClientSecret        string `json:"clientSecret"` //The api ClientSecret to Access this Service
	AccessToken         string `json:"accessToken"` //The api AccessToken to Access this service
	ServerURL           string `json:"serverURL"` //The Server address ex "https://mastodon.social
}

// Define a struct to partially match incoming JSON
type ResponseData struct {
	UserAcct	string `json:"acct"`
	DisplayName	string `json:"display_name"`
}

//Global Vars
var (
	programConfig	ProgramConfig //This object holds all program config options.
	startTime	time.Time //Holds the service's start time
	stdLogger	*log.Logger//New Logging updates should start here by removing all of these but one.
)

func main() {
	startTime = time.Now() //The program start time, used to output uptime.
	programArgs := os.Args[1:]
	configFile := programArgs[0]
	programConfig = LoadProgramConfig(configFile)

	//Required Starting Args
	//Logname
	if (programConfig.LogName == "") {
		fmt.Print("Logname missing.\n")
		os.Exit(1)
	}
	
	//Logbase
	if (programConfig.LogBase == "") {
		fmt.Print("Logbase missing\n")
		os.Exit(1)
	}
	
	//Database
	if (programConfig.DataBase == "") {
		fmt.Print("Database missing\n")
		os.Exit(1)
	}
	//Checks for the data directory, creates it w/ permissions (0755) if needed.
	if err := os.MkdirAll(programConfig.DataBase, 0755); err != nil {
		log.Fatalf("Failed to create directory: %v", err)
	}

	//ClientKey
	if (programConfig.ClientID == "") {
		fmt.Print("ClientID missing\n")
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

	//Opens the log file, if the file doesn't exist it creates the file with permissions 0644
	f, err := os.OpenFile(programConfig.LogBase + programConfig.LogName,os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err)
	}
	defer f.Close()

	stdLogger = log.New(f, "", log.LstdFlags)//Initial stdLogger writes to /dev/null/
	stdLogger.Println("Standard Logger Enabled")
	stdLogger.Printf("Program Config File: %s\n", configFile)
	
	// Setup Mastodon Client w/ credentials from conf file.
	c := mastodon.NewClient(&mastodon.Config{
		Server:       programConfig.ServerURL,
		ClientID:     programConfig.ClientID,
		ClientSecret: programConfig.ClientSecret,
		AccessToken:  programConfig.AccessToken,
	})

	// Save credentials for later usage if you wish to do so, config file, database, etc...
	fmt.Println("Set Client ID:", c.Config.ClientID)
	fmt.Println("Set Client Secret:", c.Config.ClientSecret)
	fmt.Println("Set Access Token:", c.Config.AccessToken)
	fmt.Println("Set ServerURL:", c.Config.AccessToken)

	// Get Account Info from the mastodon client.
	acc, err := c.GetAccountCurrentUser(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("\nFollowers Count:", acc.FollowersCount)
	fmt.Println("Following Count:", acc.FollowingCount)
	
	// Get Lists
	followersMap := getFollowers(acc, "")
	followingMap := getFollowing(acc, "")
	// Output Lists
	outputMap(followersMap, "Followers")
	outputMap(followingMap, "Following")

	// Compare Lists
	mutualMap := make(map[string]struct{})
	for user := range followersMap {
		if _, exists := followingMap[user]; exists {
			//Save to map
			mutualMap[user] = struct{}{}
			fmt.Println("Mutual!: ", user)
		}
	}
	// Output Mutuals
	outputMap(mutualMap, "Mutuals")
}

// outputMap outputs the contents of a map to the file directory
func outputMap(mapData map[string]struct{}, fileName string) {
	//Create file and halt if unable to write/create file
	file, err := os.OpenFile(programConfig.DataBase + fileName, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
		defer file.Close()
	if err != nil {
		fmt.Errorf("failed to create file: %w", err)
		os.Exit(1)
	}

	//For each line of data in the map write it to a line in the file
	for line := range mapData {
		//Output to terminal
		fmt.Printf("Writing %s to %s\n", line, fileName)

		//Ouput to file
		if _, err := file.WriteString(line + "\n"); err != nil {
			fmt.Errorf("failed to write to file: %w", err)
		}

	}
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

//Get All Following
//The API endpoint is: GET /api/v1/accounts/{id}/following?limit=80&max_id=nextID
//Will us an httpClient to fetch Following using the API and JSON
func getFollowing(acc *mastodon.Account, pageID string) (map[string]struct{}) {
	followingSet := make(map[string]struct{})

	//TODO: Update max and RateLimit Info using API
	//API Call
	url := fmt.Sprintf("https://mastodon.social/api/v1/accounts/%s/following?limit=80&max_id=%s", acc.ID, pageID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalf("Error creating HTTP request: %v", err)
	}

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Fatalf("Error making HTTP request: %v", err)
	}
	defer resp.Body.Close()

	//Read the Response Body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading body:", err)
	}
	
	//Unmarshal the JSON data
	var data []ResponseData
	err = json.Unmarshal(body, &data)
	if err != nil {
		fmt.Println("Error unmarshaling JSON:", err)
	}
	
	for position := range data {
		//Save to map
		followingSet[data[position].UserAcct] = struct{}{}

		//Output to terminal
		fmt.Println("Following Account:", data[position].UserAcct)

	}

	//Read and output resp.Header information
	for key, values := range resp.Header {
		if key == "Link"{
			pstring, _ := parseMaxID(values[0])
			fmt.Println("Parsed Max ID:", pstring)
			if pstring != "" {
				rFollowingSet := getFollowing(acc, pstring)
				for line := range rFollowingSet {
					followingSet[line] = struct{}{}	
				}
			}
		}
		for _, value := range values {
			fmt.Println("Other: ", key, value)
		}
	}

	return followingSet
}

// Outputs all Followers
// The API endpoint is: GET /api/v1/accounts/{id}/follower?limit=80&max_id=nextID
// Will use an httpClient to fetch Followers using the API and JSON
func getFollowers(acc *mastodon.Account, pageID string) (map[string]struct{}) {
	followerSet := make(map[string]struct{})

	//TODO: Update Max_ID and Ratelimit Info using API
	//API Call
	url := fmt.Sprintf("https://mastodon.social/api/v1/accounts/%s/followers?limit=80&max_id=%s", acc.ID, pageID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalf("Error creating HTTP request: %v", err)
	}

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Fatalf("Error making HTTP request: %v", err)
	}
	defer resp.Body.Close()
	
	//Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading body:", err)
	}	

	//Unmarshal the JSON data
	var data []ResponseData
	err = json.Unmarshal(body, &data)
	if err != nil {
		fmt.Println("Error unmarshaling JSON:", err)
	}

	for position := range data {
		//Save to map
		followerSet[data[position].UserAcct] = struct{}{}

		//Output to terminal
		fmt.Println("Follower Account:", data[position].UserAcct)
	}

	//Read and output resp.Header information
	for key, values := range resp.Header {
		if key == "Link"{
			pstring, _ := parseMaxID(values[0])
			fmt.Println("Parsed Max ID:", pstring)
			if pstring != ""{
				rFollowerSet := getFollowers(acc, pstring)
				for line := range rFollowerSet {
					followerSet[line] = struct{}{}
				}
			}
		}
		for _, value := range values {
			fmt.Println("Other: ", key, value)
		}
	}

	return followerSet
}

//Help fuction. Might be a better way to do this.
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
