package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	Port        = "8000"
	AuthURL     = "https://accounts.spotify.com/authorize"
	TokenURL    = "https://accounts.spotify.com/api/token"
	RedirectURI = "http://127.0.0.1:" + Port + "/callback"
	TokenFile   = "~/.local/state/spotify_mini/token.json"
)

type Token struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresIn    int       `json:"expires_in"`
	Expiry       time.Time `json:"expiry"`
}

func Authenticate(clientID, clientSecret string) (*Token, error) {

	token, err := loadToken()
	if err == nil {
		if time.Now().After(token.Expiry) {

			newToken, err := refreshToken(clientID, clientSecret, token.RefreshToken)
			if err == nil {
				return newToken, nil
			}

			fmt.Println("Refresh failed, restarting login flow...")
		} else {
			return token, nil
		}
	}

	return startWebLogin(clientID, clientSecret)
}

func startWebLogin(clientID, clientSecret string) (*Token, error) {
	scopes := "user-read-playback-state user-modify-playback-state"

	v := url.Values{}
	v.Set("client_id", clientID)
	v.Set("response_type", "code")
	v.Set("redirect_uri", RedirectURI)
	v.Set("scope", scopes)

	loginURL := AuthURL + "?" + v.Encode()
	fmt.Printf("Login required. Opening browser: %s\n", loginURL)

	exec.Command("xdg-open", loginURL).Start()

	codeChan := make(chan string)
	srv := &http.Server{Addr: ":" + Port}

	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		fmt.Fprintf(w, "Login Success! You can close this.")
		codeChan <- code
	})

	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			fmt.Printf("HTTP server error: %v\n", err)
		}
	}()

	code := <-codeChan
	srv.Shutdown(context.Background())

	return exchangeCode(clientID, clientSecret, code)
}

func exchangeCode(clientID, clientSecret, code string) (*Token, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", RedirectURI)
	return makeTokenRequest(clientID, clientSecret, data)
}

func refreshToken(clientID, clientSecret, refreshStr string) (*Token, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshStr)
	return makeTokenRequest(clientID, clientSecret, data)
}

func makeTokenRequest(clientID, clientSecret string, data url.Values) (*Token, error) {
	req, _ := http.NewRequest("POST", TokenURL, strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(clientID+":"+clientSecret)))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API Error: %s", resp.Status)
	}

	newToken := &Token{}
	json.NewDecoder(resp.Body).Decode(newToken)

	newToken.Expiry = time.Now().Add(time.Duration(newToken.ExpiresIn-10) * time.Second)

	if newToken.RefreshToken == "" {

		old, _ := loadToken()
		if old != nil {
			newToken.RefreshToken = old.RefreshToken
		}
	}

	saveToken(newToken)

	return newToken, nil
}

func getTokenPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configDir = filepath.Join(home, ".config")
	}

	appDir := filepath.Join(configDir, "spotify-mini")

	if err := os.MkdirAll(appDir, 0755); err != nil {
		return "", err
	}

	return filepath.Join(appDir, "token.json"), nil
}

func saveToken(t *Token) {
	path, err := getTokenPath()
	if err != nil {
		fmt.Printf("Error getting token path: %v\n", err)
		return
	}

	f, _ := os.Create(path)
	defer f.Close()
	json.NewEncoder(f).Encode(t)
}

func loadToken() (*Token, error) {
	path, err := getTokenPath()
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	t := &Token{}
	err = json.NewDecoder(f).Decode(t)
	return t, err
}
