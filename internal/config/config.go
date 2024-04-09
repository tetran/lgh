package config

import "strings"

const WorkDir = ".lgh"

type Config struct {
	ApiKey string
	Lang   string
}

var languages = map[string]string{
	"en": "English",
	"ja": "Japanese",
}

func (c *Config) FullLang() string {
	if lang, ok := languages[c.Lang]; ok {
		return lang
	}

	for k, v := range languages {
		if strings.HasPrefix(c.Lang, k) {
			return v
		}
	}

	return "English"
}
