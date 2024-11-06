package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

func sendRequest(url string) (Response, error) {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return Response{}, fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return Response{}, fmt.Errorf("non-200 status code: %d\nResponse body: %s", resp.StatusCode, body)
	}

	var response Response
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return Response{}, fmt.Errorf("error decoding JSON response: %v", err)
	}

	return response, nil
}

func getTotalWallets() (int, error) {
	response, err := sendRequest(baseURL)
	if err != nil {
		return 0, fmt.Errorf("failed to get total wallets: %v", err)
	}
	return response.Data.Total, nil
}

func getTotalPoints(url string) (int, error) {
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

	var result []int
	for _, percent := range percentForTop {
		rank := int(float64(totalUsers) * percent)
		url := fmt.Sprintf(baseURL+"?page=%d&size=1", rank)
		totalPoints, err := getTotalPoints(url)
		if err != nil {
			return nil, fmt.Errorf("failed to get total points for rank %d: %v", rank, err)
		}
		result = append(result, totalPoints)
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
