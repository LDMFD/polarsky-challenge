package quotefinder

import (
	"github.com/knadh/koanf/parsers/dotenv"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/pkg/errors"
	"os"
	"strings"
)

func loadAPIKey(path string, getenv func(string) string) (string, error) {
	config := koanf.New(".")
	if err := config.Load(file.Provider(path), dotenv.Parser()); err != nil && !os.IsNotExist(errors.Cause(err)) {
		return "", errors.Wrap(err, "load env.props")
	}
	key := strings.TrimSpace(getenv("OPENAI_API_KEY"))
	if key == "" {
		key = strings.TrimSpace(config.String("OPENAI_API_KEY"))
	}
	if key == "" {
		return "", errors.New("OPENAI_API_KEY is not set in the environment or env.props")
	}
	return key, nil
}
