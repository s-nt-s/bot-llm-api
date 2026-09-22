package httpapi

import (
	"bot-api/internal/bot"
	"bot-api/internal/config"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

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

func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", HandleRequest)
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

func HandleRequest(w http.ResponseWriter, r *http.Request) {
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

	values := getValues(r)
	ask := values.Get("ask")
	if ask == "" {
		writeError(w, http.StatusBadRequest, "ask is required and cannot be empty")
		return
	}

	botConfig, err := config.NewBotConfig(pathParts[0])
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	userSlug := pathParts[2]
	user := config.NewOptionalUser(botConfig, userSlug)

	resp := getResponse(endpoint, botConfig, user, ask)
	if resp == nil {
		if user == nil {
			user = &config.UserConfig{}
		}
		writeJSON(w, http.StatusNotFound, response{
			Bot:  botConfig.Name,
			User: user.Name,
			Ask:  ask,
		})
		return
	}
	writeJSON(w, resp.Status, resp.ToJSON())
}

func getResponse(endpoint EndpointType, botConfig *config.BotConfig, user *config.UserConfig, ask string) *config.Message {
	botManager := bot.GetBot(botConfig, user)
	if endpoint == EndpointTypeQuery {
		return botManager.DoQuery(ask)
	}
	if endpoint == EndpointTypeChat {
		return botManager.DoChat(ask)
	}
	return nil
}

func getValues(r *http.Request) url.Values {
	if r.Method == http.MethodGet {
		return r.URL.Query()
	}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			return url.Values{}
		}

		values := url.Values{}
		if ask, ok := payload["ask"].(string); ok {
			values.Set("ask", ask)
		}
		return values
	}
	if err := r.ParseForm(); err == nil {
		return r.Form
	}
	return url.Values{}
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
