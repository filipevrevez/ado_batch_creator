package models

type Task struct {
	Name        string `yaml:"name" json:"name"`
	Type        string `yaml:"type" json:"type"`
	Description string `yaml:"description" json:"description"`
	Owner       string `yaml:"owner" json:"owner"`
	State       string `yaml:"state" json:"state"`
	Priority    int    `yaml:"priority" json:"priority"`
	Estimate    int    `yaml:"estimate" json:"estimate"`
}
