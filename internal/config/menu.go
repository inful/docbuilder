package config

// Menu represents a Hugo menu item
type Menu struct {
    Name   string `yaml:"name"`
    URL    string `yaml:"url"`
    Weight int    `yaml:"weight,omitempty"`
}
