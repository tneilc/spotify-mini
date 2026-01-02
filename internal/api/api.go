package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const APIBase = "https://api.spotify.com/v1/"

type PlayerState struct {
	IsPlaying  bool     `json:"is_playing"`
	ProgressMs int      `json:"progress_ms"`
	Item       *Item    `json:"item"`
	Context    *Context `json:"context"`
}

type Context struct {
	URI  string `json:"uri"`
	Type string `json:"type"`
}

type QueueResponse struct {
	Queue []Item `json:"queue"`
}

type Item struct {
	Name       string   `json:"name"`
	URI        string   `json:"uri"`
	DurationMs int      `json:"duration_ms"`
	Artists    []Artist `json:"artists"`
}

type Artist struct {
	Name string `json:"name"`
}

func GetCurrentPlaying(token *Token) (*PlayerState, error) {
	req, _ := http.NewRequest("GET", APIBase+"me/player", nil)
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 204 {
		return nil, nil
	}

	var state PlayerState
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		return nil, err
	}
	return &state, nil
}

func GetQueue(token *Token) ([]Item, error) {
	req, _ := http.NewRequest("GET", APIBase+"me/player/queue", nil)
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var qResp QueueResponse
	if err := json.NewDecoder(resp.Body).Decode(&qResp); err != nil {
		return nil, err
	}
	return qResp.Queue, nil
}

func SendCommand(method, endpoint string, token *Token) error {
	req, _ := http.NewRequest(method, APIBase+"me/player/"+endpoint, nil)
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		return fmt.Errorf("status: %s", resp.Status)
	}
	return nil
}

func PlaySong(token *Token, uris []string) error {
	type PlayBody struct {
		URIs []string `json:"uris"`
	}
	bodyData := PlayBody{URIs: uris}
	jsonBytes, _ := json.Marshal(bodyData)
	return sendPlayRequest(token, jsonBytes)
}

func PlayContext(token *Token, contextURI, offsetTrackURI string) error {

	body := fmt.Sprintf(`{"context_uri": "%s", "offset": {"uri": "%s"}}`, contextURI, offsetTrackURI)
	return sendPlayRequest(token, []byte(body))
}

func sendPlayRequest(token *Token, body []byte) error {
	req, _ := http.NewRequest("PUT", APIBase+"me/player/play", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		return fmt.Errorf("status: %s", resp.Status)
	}
	return nil
}
