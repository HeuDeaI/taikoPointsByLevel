package main

import (
	"encoding/json"
	"fmt"
	"io"
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

var percentForTop = []float64{0.0001, 0.001, 0.005, 0.01, 0.03, 0.04, 0.06, 0.08, 0.1, 0.18, 0.26}

var client = &http.Client{Timeout: timeout}

func sendRequest(url string) (Response, error) {
	var response Response
	for attempts := 0; attempts < retryLimit; attempts++ {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return response, fmt.Errorf("error creating request: %v", err)
		}
		req.Header.Set("User-Agent", "Mozilla/5.0")

		resp, err := client.Do(req)
		if err != nil {
			if attempts < retryLimit-1 {
				time.Sleep(time.Second * time.Duration(attempts+1)) // Exponential backoff
				continue
			}
			return response, fmt.Errorf("error sending request after retries: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return response, fmt.Errorf("non-200 status code: %d\nResponse body: %s", resp.StatusCode, body)
		}

		err = decodeResponse(resp.Body, &response)
		if err != nil {
			return response, fmt.Errorf("error decoding JSON response: %v", err)
		}
		return response, nil
	}
	return response, fmt.Errorf("retries exceeded")
}

// decodeResponse is a helper function to decode a response body into a Response struct
func decodeResponse(body io.Reader, response *Response) error {
	return json.NewDecoder(body).Decode(response)
}

func getTotalWallets() (int, error) {
	response, err := sendRequest(baseURL)
	if err != nil {
		return 0, fmt.Errorf("failed to get total wallets: %v", err)
	}
	return response.Data.Total, nil
}

func getTotalPoints(rank int) (int, error) {
	url := fmt.Sprintf("%s?page=%d&size=1", baseURL, rank)
	response, err := sendRequest(url)
	if err != nil {
		return 0, fmt.Errorf("failed to get total points: %v", err)
	}

	if len(response.Data.Users) == 0 {
		return 0, fmt.Errorf("no users found in response")
	}

	return int(response.Data.Users[0].TotalScore), nil
}

func SetPointsForLevel() ([]int, error) {
	totalUsers, err := getTotalWallets()
	if err != nil {
		return nil, fmt.Errorf("failed to get total wallets: %v", err)
	}

	var wg sync.WaitGroup
	result := make([]int, len(percentForTop))
	var once sync.Once
	var finalErr error

	for i, percent := range percentForTop {
		wg.Add(1)
		go func(i int, percent float64) {
			defer wg.Done()
			rank := int(float64(totalUsers) * percent)
			totalPoints, err := getTotalPoints(rank)
			if err != nil {
				once.Do(func() { finalErr = fmt.Errorf("failed to get total points for rank %d: %v", rank, err) })
				return
			}
			result[i] = totalPoints
		}(i, percent)
	}

	wg.Wait()

	if finalErr != nil {
		return nil, finalErr
	}

	return result, nil
}

func main() {
	result, err := SetPointsForLevel()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Printf("Result: %v\n", result)
}
