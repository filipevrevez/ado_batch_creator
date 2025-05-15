package models

type UserStory struct {
	Name        string  `yaml:"name" json:"name"`
	Type        string  `yaml:"type" json:"type"`
	Description string  `yaml:"description" json:"description"`
	Owner       string  `yaml:"owner" json:"owner"`
	State       string  `yaml:"state" json:"state"`
	Priority    int     `yaml:"priority" json:"priority"`
	Area        string  `yaml:"area" json:"area"`
	Path        string  `yaml:"path" json:"path"`
	Tasks       []Task  `yaml:"tasks" json:"tasks"`
	Iteraction  *string `yaml:"iteraction" json:"iteraction"`
	Team        string  `yaml:"team" json:"team"`
}
