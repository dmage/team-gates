package config

import (
	"io/ioutil"
	"os"

	"sigs.k8s.io/yaml"
)

type Team struct {
	Name       string   `json:"name"`
	Peeps      int      `json:"peeps"`
	Components []string `json:"components"`
}

type Config struct {
	Teams []Team `json:"teams"`
}

func (c *Config) Team(name string) (*Team, bool) {
	for _, team := range c.Teams {
		if team.Name == name {
			return &team, true
		}
	}
	return nil, false
}

func FromFile(filename string) (*Config, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	cfg := &Config{}
	err = yaml.Unmarshal(data, cfg)
	return cfg, err
}
