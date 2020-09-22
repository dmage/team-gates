package bugzilla

import (
	"encoding/json"
	"net/http"
)

type Client struct {
	url string
}

func NewClient(url string) *Client {
	return &Client{
		url: url,
	}
}

func (c *Client) SearchBugs(query string) ([]Bug, error) {
	resp, err := http.Get(c.url + "bug?" + query)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var v struct {
		Bugs []Bug `json:"bugs"`
	}
	err = json.NewDecoder(resp.Body).Decode(&v)
	return v.Bugs, err
}
