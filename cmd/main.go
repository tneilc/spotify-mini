package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"spotify-mini/internal/api"
	"spotify-mini/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

const StatusFile = "/tmp/spotify-mini.json"

func main() {

	mode := flag.String("mode", "status", "Mode: status, ui, or daemon")
	cmd := flag.String("cmd", "", "Command: next, prev, play, pause, toggle")
	flag.Parse()

	clientID := os.Getenv("SPOTIFY_ID")
	clientSecret := os.Getenv("SPOTIFY_SECRET")
	if clientID == "" {
		fmt.Println("Error: Missing SPOTIFY_ID/SECRET")
		os.Exit(1)
	}

	if *cmd != "" {

		token, err := api.Authenticate(clientID, clientSecret)
		if err != nil {
			fmt.Println("Auth error:", err)
			return
		}
		handleCommand(token, *cmd)

		exec.Command("pkill", "-USR1", "-f", "spotify-mini -mode daemon").Run()
		return
	}

	switch *mode {
	case "daemon":
		runDaemon(clientID, clientSecret)
	case "ui":

		token, err := api.Authenticate(clientID, clientSecret)
		if err != nil {
			panic(err)
		}
		p := tea.NewProgram(ui.InitialModel(token), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}
	default:

		token, _ := api.Authenticate(clientID, clientSecret)
		writeStatus(token, true)
	}
}

func runDaemon(clientID, clientSecret string) {

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGUSR1)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	update := func() {

		token, err := api.Authenticate(clientID, clientSecret)
		if err != nil {
			return
		}
		writeStatusToFile(token)
	}

	update()

	for {
		select {
		case <-ticker.C:
			update()
		case <-sigs:
			update()
		}
	}
}

func writeStatusToFile(token *api.Token) {
	jsonOut := getStatusJSON(token)

	f, _ := os.Create("/tmp/spotify-mini.json.tmp")
	f.WriteString(jsonOut)
	f.Close()
	os.Rename("/tmp/spotify-mini.json.tmp", StatusFile)
}

func writeStatus(token *api.Token, printToStdout bool) {
	s := getStatusJSON(token)
	if printToStdout {
		fmt.Println(s)
	}
}

func getStatusJSON(token *api.Token) string {
	state, err := api.GetCurrentPlaying(token)

	if err != nil || state == nil || state.Item == nil {
		return `{"text": "", "class": "spotify", "alt": "stopped"}`
	}

	icon := ""
	if !state.IsPlaying {
		icon = ""
	}
	artist := state.Item.Artists[0].Name
	title := state.Item.Name

	return fmt.Sprintf(`{"text": "%s %s", "class": "spotify", "tooltip": "%s by %s", "alt": "%s"}`,
		icon, title, title, artist, func() string {
			if state.IsPlaying {
				return "playing"
			} else {
				return "paused"
			}
		}())
}

func handleCommand(token *api.Token, cmd string) {
	switch cmd {
	case "next":
		api.SendCommand("POST", "next", token)
	case "prev":
		api.SendCommand("POST", "previous", token)
	case "play":
		api.SendCommand("PUT", "play", token)
	case "pause":
		api.SendCommand("PUT", "pause", token)
	case "toggle":
		state, _ := api.GetCurrentPlaying(token)
		if state != nil && state.IsPlaying {
			api.SendCommand("PUT", "pause", token)
		} else {
			api.SendCommand("PUT", "play", token)
		}
	}
}
