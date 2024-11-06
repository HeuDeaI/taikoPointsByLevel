package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

func sendRequest(url string) (Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return Response{}, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil {
		return Response{}, err
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

func getTotalWallets(baseURL string) (int, error) {
	response, err := sendRequest(baseURL)
	if err != nil {
		return 0, fmt.Errorf("failed to get response: %v", err)
	}

	return response.Data.Total, nil
}

func getTotalPoints(url string) (int, error) {
	response, err := sendRequest(url)
	if err != nil {
		return 0, fmt.Errorf("failed to get response: %v", err)
	}

	return int(response.Data.Users[0].TotalScore), nil

}

func SetPointsForLevel(baseURL string, totalUsers int) ([]int, error) {
	urlParams := "?page=%d&size=1"
	percentForTop := []float64{0.0001, 0.001, 0.005, 0.01, 0.03, 0.04, 0.06, 0.08, 0.1, 0.18, 0.26}
	result := []int{}

	for i := 0; i < len(percentForTop); i++ {
		rank := int(float64(totalUsers) * percentForTop[i])
		url := fmt.Sprintf(baseURL+urlParams, rank)
		totalPoints, err := getTotalPoints(url)
		if err != nil {
			return []int{}, fmt.Errorf("failed to get total points: %v", err)
		}

		result = append(result, totalPoints)
	}

	return result, nil
}

func main() {
	baseURL := "https://trailblazer.mainnet.taiko.xyz/s2/v2/leaderboard/user"

	total, err := getTotalWallets(baseURL)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	result, err := SetPointsForLevel(baseURL, 756628)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Printf("Total wallets: %d\n", total)
	fmt.Printf("Result: %d\n", result)
}
