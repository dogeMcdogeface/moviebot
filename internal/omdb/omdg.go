package omdb

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
)

type OMDbClient struct {
	APIKey string
}

type SearchResult struct {
	Title  string `json:"Title"`
	Year   string `json:"Year"`
	ImdbID string `json:"imdbID"`
	Type   string `json:"Type"`
	Poster string `json:"Poster"`
}

type SearchResponse struct {
	Search       []SearchResult `json:"Search"`
	TotalResults string         `json:"totalResults"`
	Response     string         `json:"Response"`
	Error        string         `json:"Error,omitempty"`
}

func NewClient(apiKey string) *OMDbClient {
	if apiKey == "" {
		log.Fatal("[OMDb] API key not set")
	}
	return &OMDbClient{APIKey: apiKey}
}

// Test if the API key works
func (c *OMDbClient) TestKey() bool {
	log.Println("[OMDb] Testing API key...")
	resp, err := http.Get(fmt.Sprintf("http://www.omdbapi.com/?apikey=%s&s=test", c.APIKey))
	if err != nil {
		log.Println("[OMDb] Error contacting OMDb:", err)
		return false
	}
	defer resp.Body.Close()

	var r SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		log.Println("[OMDb] Error decoding response:", err)
		return false
	}

	if r.Response != "True" && r.Error == "Invalid API key!" {
		log.Println("[OMDb] Invalid API key")
		return false
	}

	log.Println("[OMDb] API key appears valid")
	return true
}

// Search for a movie by title
func (c *OMDbClient) Search(title string) ([]SearchResult, error) {
	log.Printf("[OMDb] Searching for: %s\n", title)
	baseURL := "http://www.omdbapi.com/"
	params := url.Values{}
	params.Set("apikey", c.APIKey)
	params.Set("s", title)

	fullURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())
	resp, err := http.Get(fullURL)
	if err != nil {
		log.Println("[OMDb] HTTP error:", err)
		return nil, err
	}
	defer resp.Body.Close()

	var r SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		log.Println("[OMDb] JSON decode error:", err)
		return nil, err
	}

	if r.Response != "True" {
		log.Println("[OMDb] No results found or error:", r.Error)
		return nil, fmt.Errorf("OMDb error: %s", r.Error)
	}

	log.Printf("[OMDb] Found %d results\n", len(r.Search))
	return r.Search, nil
}