package config

const WorkDir = ".lgh"

type Config struct {
	ApiKey string
	Lang   string
}

var Languages = map[string]string{
	"en": "English",
	"ja": "Japanese",
}

func (c *Config) FullLang() string {
	return Languages[c.Lang]
}
