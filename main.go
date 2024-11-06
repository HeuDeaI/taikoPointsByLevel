package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

type User struct {
	Rank       int     `json:"rank"`
	Address    string  `json:"address"`
	Score      float64 `json:"score"`
	Multiplier int     `json:"multiplier"`
	TotalScore float64 `json:"totalScore"`
}

type Data struct {
	Users      []User `json:"items"`
	Page       int    `json:"page"`
	Size       int    `json:"size"`
	Total      int    `json:"total"`
	TotalPages int    `json:"total_pages"`
}

type Response struct {
	Data        Data  `json:"data"`
	LastUpdated int64 `json:"lastUpdated"`
}

const (
	baseURL    = "https://trailblazer.mainnet.taiko.xyz/s2/v2/leaderboard/user"
	timeout    = 10 * time.Second
	retryLimit = 3
)

var (
	client         = &http.Client{Timeout: timeout}
	topPercentages = []float64{
		0.0001, 0.001, 0.005, 0.01, 0.03, 0.04, 0.06, 0.08, 0.1, 0.18, 0.26,
	}
)

func fetchResponse(url string) (Response, error) {
	var response Response
	for attempt := 0; attempt < retryLimit; attempt++ {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			log.Printf("Failed to create request: %v", err)
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0")

		resp, err := client.Do(req)
		if err != nil {
			if attempt < retryLimit-1 {
				time.Sleep(time.Second * time.Duration(attempt+1))
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

func parseJSONResponse(body io.Reader, response *Response) error {
	return json.NewDecoder(body).Decode(response)
}

func getTotalWallets() (int, error) {
	response, err := fetchResponse(baseURL)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch total wallets: %v", err)
	}
	return response.Data.Total, nil
}

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

func calculatePointsForTopUsers() ([]int, error) {
	totalUsers, err := getTotalWallets()
	if err != nil {
		return nil, fmt.Errorf("failed to get total wallets: %v", err)
	}

	var wg sync.WaitGroup
	points := make([]int, len(topPercentages))
	errs := make([]error, len(topPercentages))

	for i, percentage := range topPercentages {
		wg.Add(1)
		go func(i int, percentage float64) {
			defer wg.Done()
			rank := int(float64(totalUsers) * percentage)
			totalPoints, err := getUserTotalPoints(rank)
			if err != nil {
				errs[i] = fmt.Errorf("failed to get total points for rank %d: %v", rank, err)
				return
			}
			points[i] = totalPoints
		}(i, percentage)
	}

	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return nil, fmt.Errorf("error calculating points: %v", err)
		}
	}

	return points, nil
}

func main() {
	points, err := calculatePointsForTopUsers()
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Printf("Points for top ranks: %v\n", points)
}
