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
	DataName	    string `json:"dataName"`//The file name used for program data.
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
	//Dataname
	if (programConfig.DataName == "") {
		fmt.Print("Dataname missing\n")
		os.Exit(1)
	}
	//Database
	if (programConfig.DataBase == "") {
		fmt.Print("Database missing\n")
		os.Exit(1)
	}

	//ClientID
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
	if _, err := os.Stat(programConfig.LogBase); os.IsNotExist(err) {
	    os.Mkdir(programConfig.LogBase, 0755)
	}

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
	
	//Account used for
	getFollowers(acc, "")
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

//Output all Following
//The API endpoint is: GET /api/v1/accounts/{id}/following?limit=80&max_id=nextID
// Will us an httpClient to fetch Following using the API and JSON
func getFollowing(acc *mastodon.Account, pageID string){
	//TODO: Update max and RateLimit Info using API
	//API Call
	//Read the Response Body
	//Unmarshal the JSON data
	//Output to file
	//Read and output resp.Header information
}

// Outputs all Followers
// The API endpoint is: GET /api/v1/accounts/{id}/follower?limit=80&max_id=nextID
// Will use an httpClient to fetch Followers using the API and JSON
func getFollowers(acc *mastodon.Account, pageID string){

	//TODO: Update Max_ID and Ratelimit Info using API
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
		os.Exit(1)
	}	

	//Unmarshal the JSON data
	var data []ResponseData
	err = json.Unmarshal(body, &data)
	if err != nil {
		fmt.Println("Error unmarshaling JSON:", err)
		os.Exit(1)
	}

	//Output to file
	file, err := os.OpenFile(programConfig.DataBase + "Followers", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
		defer file.Close()
	if err != nil {
		fmt.Errorf("failed to create file: %w", err)
		os.Exit(1)
	}

	for position := range data {
		//Output to terminal
		fmt.Println("Account Location:", data[position].UserAcct)

		if _, err := file.WriteString(data[position].UserAcct + "\n"); err != nil {
			fmt.Errorf("failed to write to file: %w", err)
			os.Exit(1)
		}

	}

	log.Printf("Followers list written to %s / %s", programConfig.DataBase, programConfig.DataName)
	
	//Read and output resp.Header information
	for key, values := range resp.Header {
		if key == "Link"{
			pstring, err := parseMaxID(values[0])
			if err != nil {
				log.Fatalf("Error with parse MaxID: %v", err)
			}
			fmt.Println("Parsed Max ID:", pstring)
			getFollowers(acc, pstring)
		}
		for _, value := range values {
			fmt.Println("Other: ", key, value)
		}
	}
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
