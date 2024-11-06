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
	Rank       int     `json:"rank"`
	Address    string  `json:"address"`
	Score      float64 `json:"score"`
	Multiplier int     `json:"multiplier"`
	TotalScore float64 `json:"totalScore"`
}

// Data represents the data returned from the leaderboard API
type Data struct {
	Users      []User `json:"items"`
	Page       int    `json:"page"`
	Size       int    `json:"size"`
	Total      int    `json:"total"`
	TotalPages int    `json:"total_pages"`
}

// Response represents the full response structure from the API
type Response struct {
	Data        Data  `json:"data"`
	LastUpdated int64 `json:"lastUpdated"`
}

const (
	baseURL    = "https://trailblazer.mainnet.taiko.xyz/s2/v2/leaderboard/user"
	timeout    = 10 * time.Second
	retryLimit = 3
)

// Percentages for the top leaderboard users
var topPercentages = []float64{
	0.0001, 0.001, 0.005, 0.01, 0.03, 0.04, 0.06, 0.08, 0.1, 0.18, 0.26,
}

var client = &http.Client{Timeout: timeout}

// fetchResponse sends a GET request to the given URL and returns the response
func fetchResponse(url string) (Response, error) {
	var response Response
	for attempt := 0; attempt < retryLimit; attempt++ {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return response, fmt.Errorf("failed to create request: %v", err)
		}
		req.Header.Set("User-Agent", "Mozilla/5.0")

		resp, err := client.Do(req)
		if err != nil {
			if attempt < retryLimit-1 {
				time.Sleep(time.Second * time.Duration(attempt+1)) // Exponential backoff
				continue
			}
			return response, fmt.Errorf("failed to send request after retries: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return response, fmt.Errorf("unexpected status code: %d\nResponse body: %s", resp.StatusCode, body)
		}

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
	return json.NewDecoder(body).Decode(response)
}

// getTotalWallets fetches the total number of wallets from the leaderboard API
func getTotalWallets() (int, error) {
	response, err := fetchResponse(baseURL)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch total wallets: %v", err)
	}
	return response.Data.Total, nil
}

// getUserTotalPoints fetches the total points for a user at a specific rank
func getUserTotalPoints(rank int) (int, error) {
	url := fmt.Sprintf("%s?page=%d&size=1", baseURL, rank)
	response, err := fetchResponse(url)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch total points: %v", err)
	}

	if len(response.Data.Users) == 0 {
		return 0, fmt.Errorf("no users found in response")
	}

	return int(response.Data.Users[0].TotalScore), nil
}

// calculatePointsForTopUsers calculates the total points at specific ranks based on the percentage of total users
func calculatePointsForTopUsers() ([]int, error) {
	totalUsers, err := getTotalWallets()
	if err != nil {
		return nil, fmt.Errorf("failed to get total wallets: %v", err)
	}

	var wg sync.WaitGroup
	points := make([]int, len(topPercentages))
	var once sync.Once
	var finalError error

	for i, percentage := range topPercentages {
		wg.Add(1)
		go func(i int, percentage float64) {
			defer wg.Done()
			rank := int(float64(totalUsers) * percentage)
			totalPoints, err := getUserTotalPoints(rank)
			if err != nil {
				once.Do(func() { finalError = fmt.Errorf("failed to get total points for rank %d: %v", rank, err) })
				return
			}
			points[i] = totalPoints
		}(i, percentage)
	}

	wg.Wait()

	if finalError != nil {
		return nil, finalError
	}

	return points, nil
}

func main() {
	points, err := calculatePointsForTopUsers()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Printf("Points for top ranks: %v\n", points)
}
