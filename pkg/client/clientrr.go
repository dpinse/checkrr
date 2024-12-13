package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/soheilrt/checkrr/pkg/config"
)

const (
	queuePathV1           = "/api/v1/queue"
	queueDeleteBulkPathV1 = "/api/v1/queue/bulk"
	queuePathV3           = "/api/v3/queue"
	queueDeleteBulkPathV3 = "/api/v3/queue/bulk"
)

type ClientRR struct {
	name    string
	key     string
	host    string
	options config.Options
}

func NewClientRR(host, apiKey string, name string, options config.Options) *ClientRR {
	return &ClientRR{
		name:    name,
		key:     apiKey,
		host:    host,
		options: options,
	}
}

func (c *ClientRR) getQueuePath() string {
	if c.name == "lidarr" {
		return queuePathV1
	}
	return queuePathV3
}

func (c *ClientRR) getQueueDeletePath() string {
	if c.name == "lidarr" {
		return queueDeleteBulkPathV1
	}
	return queueDeleteBulkPathV3
}

func (c *ClientRR) FetchDownloads() ([]Download, error) {
	var downloads []Download

	for page := 1; ; page++ {
		pageDownloads, totalRecords, err := c.fetchDownloadPage(page)
		if err != nil {
			return nil, err
		}
		downloads = append(downloads, pageDownloads...)

		if len(downloads) >= totalRecords {
			break
		}
	}

	return downloads, nil
}

func (c *ClientRR) fetchDownloadPage(page int) ([]Download, int, error) {
	client := &http.Client{}
	queuePath := c.getQueuePath() // Dynamically choose the path

	req, err := http.NewRequest("GET", c.host+queuePath, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("X-Api-Key", c.key)

	// Add page query parameter
	q := req.URL.Query()
	q.Add("page", strconv.Itoa(page))
	req.URL.RawQuery = q.Encode() // Important: this actually sets the query string

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("Unexpected Status Code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}

	var response Response
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, 0, err
	}

	return response.Records, response.TotalRecords, nil
}

func (c *ClientRR) DeleteFromQueue(ids []int) error {
	client := &http.Client{}
	deletePath := c.getQueueDeletePath() // Dynamically choose the delete path

	req, err := http.NewRequest("DELETE", c.host+deletePath, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Api-Key", c.key)
	req.Header.Set("Content-Type", "application/json")

	q := req.URL.Query()
	q.Add("removeFromClient", strconv.FormatBool(!c.options.KeepInClient))
	q.Add("blocklist", strconv.FormatBool(c.options.BlockList))
	q.Add("skipRedownload", strconv.FormatBool(c.options.SkipRedownload))
	req.URL.RawQuery = q.Encode() // Important: this actually sets the query string

	body, err := json.Marshal(struct {
		Ids []int `json:"ids"`
	}{Ids: ids})
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	req.Body = io.NopCloser(bytes.NewReader(body))

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
