package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Node struct {
		URL string `yaml:"url"`
	} `yaml:"node"`
	Miner struct {
		Address string `yaml:"address"`
		Workers int    `yaml:"workers"`
	} `yaml:"miner"`
}

func loadConfig(path string) Config {
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	var cfg Config
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		panic(err)
	}
	return cfg
}
