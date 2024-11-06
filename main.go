package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// User represents a user in the leaderboard
type User struct {
	Rank       int     `json:"rank"`       // Rank of the user in the leaderboard
	Address    string  `json:"address"`    // Wallet address of the user
	Score      float64 `json:"score"`      // User's score
	Multiplier int     `json:"multiplier"` // Multiplier applied to the score
	TotalScore float64 `json:"totalScore"` // Total score after applying the multiplier
}

// Data represents the data returned from the leaderboard API
type Data struct {
	Users      []User `json:"items"`       // List of users in the response
	Page       int    `json:"page"`        // Current page number in the API response
	Size       int    `json:"size"`        // Number of users per page in the API response
	Total      int    `json:"total"`       // Total number of users in the leaderboard
	TotalPages int    `json:"total_pages"` // Total number of pages in the API response
}

// Response represents the full response structure from the API
type Response struct {
	Data        Data  `json:"data"`        // Data of users in the leaderboard
	LastUpdated int64 `json:"lastUpdated"` // Last update timestamp of the leaderboard data
}

const (
	baseURL    = "https://trailblazer.mainnet.taiko.xyz/s2/v2/leaderboard/user" // API URL for the leaderboard
	timeout    = 10 * time.Second                                               // HTTP request timeout duration
	retryLimit = 3                                                              // Number of retries in case of failure
)

// Percentages for the top leaderboard users
var topPercentages = []float64{
	0.0001, 0.001, 0.005, 0.01, 0.03, 0.04, 0.06, 0.08, 0.1, 0.18, 0.26, // Top percentage ranks to calculate
}

var client = &http.Client{Timeout: timeout} // HTTP client with a timeout configuration

// fetchResponse sends a GET request to the given URL and returns the response
func fetchResponse(url string) (Response, error) {
	var response Response
	// Retry logic for handling transient errors
	for attempt := 0; attempt < retryLimit; attempt++ {
		// Create the HTTP request
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return response, fmt.Errorf("failed to create request: %v", err)
		}
		req.Header.Set("User-Agent", "Mozilla/5.0") // Set a common user-agent header

		// Send the HTTP request
		resp, err := client.Do(req)
		if err != nil {
			// Exponential backoff on retries
			if attempt < retryLimit-1 {
				time.Sleep(time.Second * time.Duration(attempt+1))
				continue
			}
			return response, fmt.Errorf("failed to send request after retries: %v", err)
		}
		defer resp.Body.Close()

		// Check the response status code
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return response, fmt.Errorf("unexpected status code: %d\nResponse body: %s", resp.StatusCode, body)
		}

		// Parse the JSON response
		err = parseJSONResponse(resp.Body, &response)
		if err != nil {
			return response, fmt.Errorf("failed to decode JSON response: %v", err)
		}
		return response, nil
	}
	return response, fmt.Errorf("retries exceeded")
}

// parseJSONResponse decodes the response body into the Response struct
func parseJSONResponse(body io.Reader, response *Response) error {
	return json.NewDecoder(body).Decode(response) // Decode the JSON into the provided response struct
}

// getTotalWallets fetches the total number of wallets from the leaderboard API
func getTotalWallets() (int, error) {
	// Fetch response from API
	response, err := fetchResponse(baseURL)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch total wallets: %v", err)
	}
	return response.Data.Total, nil // Return the total number of wallets
}

// getUserTotalPoints fetches the total points for a user at a specific rank
func getUserTotalPoints(rank int) (int, error) {
	// Create URL with pagination to fetch data for the specific rank
	url := fmt.Sprintf("%s?page=%d&size=1", baseURL, rank)
	response, err := fetchResponse(url)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch total points: %v", err)
	}

	if len(response.Data.Users) == 0 {
		return 0, fmt.Errorf("no users found in response")
	}

	// Return the total score for the user at the specified rank
	return int(response.Data.Users[0].TotalScore), nil
}

// calculatePointsForTopUsers calculates the total points at specific ranks based on the percentage of total users
func calculatePointsForTopUsers() ([]int, error) {
	// Get the total number of wallets (users)
	totalUsers, err := getTotalWallets()
	if err != nil {
		return nil, fmt.Errorf("failed to get total wallets: %v", err)
	}

	var wg sync.WaitGroup                      // Wait group to manage concurrent goroutines
	points := make([]int, len(topPercentages)) // Slice to store points for each percentage rank
	var once sync.Once                         // Ensure error is only assigned once
	var finalError error

	// Calculate points for each rank based on percentage
	for i, percentage := range topPercentages {
		wg.Add(1) // Increment wait group counter
		go func(i int, percentage float64) {
			defer wg.Done() // Decrement wait group counter on goroutine completion
			// Calculate rank based on percentage
			rank := int(float64(totalUsers) * percentage)
			totalPoints, err := getUserTotalPoints(rank)
			if err != nil {
				// If error occurs, capture it using sync.Once to ensure it's done only once
				once.Do(func() { finalError = fmt.Errorf("failed to get total points for rank %d: %v", rank, err) })
				return
			}
			points[i] = totalPoints // Store points for the current percentage rank
		}(i, percentage)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// If there was any error during concurrent execution, return it
	if finalError != nil {
		return nil, finalError
	}

	return points, nil // Return the calculated points for top ranks
}

func main() {
	// Calculate points for top users
	points, err := calculatePointsForTopUsers()
	if err != nil {
		fmt.Println("Error:", err) // Print error if any occurs
		return
	}

	// Print the calculated points for top ranks
	fmt.Printf("Points for top ranks: %v\n", points)
}
