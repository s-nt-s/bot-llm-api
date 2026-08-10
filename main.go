package main

import (
	"bot-api/types"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

type request struct {
	Bot  string `json:"bot,omitempty"`
	User string `json:"user,omitempty"`
	Ask  string `json:"ask"`
}

type response struct {
	Bot  string `json:"bot"`
	User string `json:"user,omitempty"`
	Ask  string `json:"ask"`
}

type EndpointType string

const (
	EndpointTypeQuery EndpointType = "query"
	EndpointTypeChat  EndpointType = "chat"
)

func (t EndpointType) Valid() bool {
	switch t {
	case EndpointTypeQuery, EndpointTypeChat:
		return true
	default:
		return false
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	address := getEnv("ADDRESS", "127.0.0.1:8080")

	server := &http.Server{
		Addr:    address,
		Handler: newRouter(),
	}

	log.Printf("bot-api listening on %s", address)
	log.Fatal(server.ListenAndServe())
}

func newRouter() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleRequest)
	return mux
}

func splitPath(r *http.Request, minSize int) []string {
	path := strings.Trim(r.URL.Path, "/")
	pathParts := strings.Split(path, "/")
	for len(pathParts) < minSize {
		pathParts = append(pathParts, "")
	}
	return pathParts
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "only GET and POST requests are allowed")
		return
	}

	pathParts := splitPath(r, 3)

	endpoint := EndpointType(pathParts[1])

	if !endpoint.Valid() {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid endpoint: %q", endpoint))
		return
	}

	ask, err := readAsk(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	bot, err := types.NewBot(pathParts[0])
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	userSlug := pathParts[2]
	user := types.NewOptionalUser(bot, userSlug)

	resp := getResponse(endpoint, bot, user, ask)
	if resp == nil {
		if user == nil {
			user = &types.UserConfig{}
		}
		writeJSON(w, http.StatusNotFound, response{
			Bot:  bot.Name,
			User: user.Name,
			Ask:  ask,
		})
		return
	}
	writeJSON(w, resp.Status, resp)
}

func getResponse(endpoint EndpointType, bot *types.BotConfig, user *types.UserConfig, ask string) *types.Message {
	botManager := &types.Bot{
		Config: bot,
		User:   user,
	}
	if endpoint == EndpointTypeQuery {
		return botManager.DoQuery(ask)
	}
	if endpoint == EndpointTypeChat {
		return botManager.DoChat(ask)
	}
	return nil
}

func readAsk(r *http.Request) (string, error) {
	if r.Method == http.MethodGet {
		if ask := strings.TrimSpace(r.URL.Query().Get("ask")); ask != "" {
			return ask, nil
		}
		return "", errors.New("ask is required and cannot be empty")
	}

	contentType := strings.ToLower(strings.Split(r.Header.Get("Content-Type"), ";")[0])
	if contentType == "application/json" {
		var body request
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return "", errors.New("the JSON body is invalid")
		}
		if ask := strings.TrimSpace(body.Ask); ask != "" {
			return ask, nil
		}
		return "", errors.New("ask is required and cannot be empty")
	}

	if err := r.ParseForm(); err != nil {
		return "", errors.New("the form is invalid")
	}
	if ask := strings.TrimSpace(r.Form.Get("ask")); ask != "" {
		return ask, nil
	}
	return "", errors.New("ask is required and cannot be empty")
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{
		"status": status,
		"error":  message,
	})
}
