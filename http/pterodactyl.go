package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"ozorg.xyz/flapjack/cache"
)

type Pterodactyl struct {
	URL    string
	APIKey string
}

type ServerDetailResponse struct {
	Object string `json:"object"`

	Attributes struct {
		ServerOwner bool   `json:"server_owner"`
		Identifier  string `json:"identifier"`
		InternalID  int    `json:"internal_id"`
		UUID        string `json:"uuid"`
		Name        string `json:"name"`
		Node        string `json:"node"`

		IsNodeUnderMaintenance bool `json:"is_node_under_maintenance"`

		SFTPDetails struct {
			IP   string `json:"ip"`
			Port int    `json:"port"`
		} `json:"sftp_details"`

		Description string `json:"description"`

		Limits struct {
			Memory      int  `json:"memory"`
			Swap        int  `json:"swap"`
			Disk        int  `json:"disk"`
			IO          int  `json:"io"`
			CPU         int  `json:"cpu"`
			Threads     any  `json:"threads"`
			OOMDisabled bool `json:"oom_disabled"`
		} `json:"limits"`

		Invocation  string `json:"invocation"`
		DockerImage string `json:"docker_image"`

		EggFeatures []string `json:"egg_features"`

		FeatureLimits struct {
			Databases   int `json:"databases"`
			Allocations int `json:"allocations"`
			Backups     int `json:"backups"`
		} `json:"feature_limits"`

		Status string `json:"status"`

		IsSuspended    bool `json:"is_suspended"`
		IsInstalling   bool `json:"is_installing"`
		IsTransferring bool `json:"is_transferring"`
	} `json:"attributes"`

	Meta struct {
		IsServerOwner   bool     `json:"is_server_owner"`
		UserPermissions []string `json:"user_permissions"`
	} `json:"meta"`
}

type ServerListResponse struct {
	Object string `json:"object"`

	Data []struct {
		Object string `json:"object"`

		Attributes struct {
			Identifier     string `json:"identifier"`
			Name           string `json:"name"`
			Description    string `json:"description"`
			Node           string `json:"node"`
			IsSuspended    bool   `json:"is_suspended"`
			IsInstalling   bool   `json:"is_installing"`
			IsTransferring bool   `json:"is_transferring"`

			Limits struct {
				Memory int `json:"memory"`
				Disk   int `json:"disk"`
				CPU    int `json:"cpu"`
			} `json:"limits"`
		} `json:"attributes"`
	} `json:"data"`
}

func NewPterodactyl(store *cache.Store) *Pterodactyl {
	url, urlErr := store.Get("PTERODACTYL_URL")
	apiKey, keyErr := store.Get("PTERODACTYL_API_KEY")

	if urlErr != nil || keyErr != nil || url == "" || apiKey == "" {
		return nil
	}

	return &Pterodactyl{
		URL:    url,
		APIKey: apiKey,
	}
}

func (p *Pterodactyl) GetServers() (*ServerListResponse, error) {
	req, err := http.NewRequest(
		"GET",
		fmt.Sprintf("%s/api/client", p.URL),
		nil,
	)

	if err != nil {
		return nil, err
	}

	req.Header.Set(
		"Authorization",
		fmt.Sprintf("Bearer %s", p.APIKey),
	)

	req.Header.Set(
		"Accept",
		"Application/vnd.pterodactyl.v1+json",
	)

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	client := &http.Client{}

	resp, err := client.Do(req)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var data ServerListResponse

	err = json.NewDecoder(resp.Body).Decode(&data)

	if err != nil {
		return nil, err
	}

	return &data, nil
}

type PowerAction string

const (
	Start   PowerAction = "start"
	Stop    PowerAction = "stop"
	Restart PowerAction = "restart"
	Kill    PowerAction = "kill"
)

type PowerRequest struct {
	Signal PowerAction `json:"signal"`
}

func (p *Pterodactyl) Power(
	serverID string,
	action PowerAction,
) error {
	body, err := json.Marshal(
		PowerRequest{
			Signal: action,
		},
	)

	if err != nil {
		return err
	}

	req, err := http.NewRequest(
		"POST",

		fmt.Sprintf(
			"%s/api/client/servers/%s/power",
			p.URL,
			serverID,
		),

		bytes.NewBuffer(body),
	)

	if err != nil {
		return err
	}

	req.Header.Set(
		"Authorization",
		fmt.Sprintf("Bearer %s", p.APIKey),
	)

	req.Header.Set(
		"Accept",
		"Application/vnd.pterodactyl.v1+json",
	)

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	client := &http.Client{}

	resp, err := client.Do(req)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		return fmt.Errorf(
			"pterodactyl returned status %d",
			resp.StatusCode,
		)
	}

	return nil
}

type WebsocketResponse struct {
	Data struct {
		Token  string `json:"token"`
		Socket string `json:"socket"`
	} `json:"data"`
}

func (p *Pterodactyl) GetConsole(
	serverID string,
	duration time.Duration,
) ([]string, error) {
	req, err := http.NewRequest(
		"GET",

		fmt.Sprintf(
			"%s/api/client/servers/%s/websocket",
			p.URL,
			serverID,
		),

		nil,
	)

	if err != nil {
		return nil, err
	}

	req.Header.Set(
		"Authorization",
		fmt.Sprintf("Bearer %s", p.APIKey),
	)

	req.Header.Set(
		"Accept",
		"Application/vnd.pterodactyl.v1+json",
	)

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	client := &http.Client{}

	resp, err := client.Do(req)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var wsData WebsocketResponse

	err = json.NewDecoder(resp.Body).Decode(&wsData)

	if err != nil {
		return nil, err
	}

	conn, _, err := websocket.DefaultDialer.Dial(
		wsData.Data.Socket,
		http.Header{
			"Origin": []string{p.URL},
		},
	)

	if err != nil {
		return nil, err
	}

	defer conn.Close()

	authPayload := map[string]interface{}{
		"event": "auth",
		"args": []string{
			wsData.Data.Token,
		},
	}

	err = conn.WriteJSON(authPayload)

	if err != nil {
		return nil, err
	}

	var logs []string

	timeout := time.After(duration)

	for {
		select {
		case <-timeout:
			return logs, nil

		default:
			_, message, err := conn.ReadMessage()

			if err != nil {
				return logs, err
			}

			var data map[string]interface{}

			err = json.Unmarshal(message, &data)

			if err != nil {
				continue
			}

			event, ok := data["event"].(string)

			if !ok {
				continue
			}

			if event == "console output" {
				args, ok := data["args"].([]interface{})

				if !ok || len(args) == 0 {
					continue
				}

				logs = append(
					logs,
					fmt.Sprintf("%v", args[0]),
				)
			}
		}
	}
}

func (p *Pterodactyl) GetServer(serverID string) (*ServerDetailResponse, error) {
	req, err := http.NewRequest(
		"GET",
		fmt.Sprintf("%s/api/client/servers/%s", p.URL, serverID),
		nil,
	)

	if err != nil {
		return nil, err
	}

	req.Header.Set(
		"Authorization",
		fmt.Sprintf("Bearer %s", p.APIKey),
	)

	req.Header.Set(
		"Accept",
		"Application/vnd.pterodactyl.v1+json",
	)

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var data ServerDetailResponse

	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return nil, err
	}

	return &data, nil
}
