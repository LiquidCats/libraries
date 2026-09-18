package cfg

import (
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
	"github.com/kelseyhightower/envconfig"
)

type loadCfg struct {
	prefix   string
	filePath string
}

type Opt func(*loadCfg)

func WithPrefix(prefix string) Opt {
	return func(cfg *loadCfg) {
		cfg.prefix = prefix
	}
}

func WithFilePath(filePath string) Opt {
	return func(cfg *loadCfg) {
		cfg.filePath = filePath
	}
}

func Load[Cfg any](opts ...Opt) (result Cfg, err error) {
	cfg := &loadCfg{}
	for _, opt := range opts {
		opt(cfg)
	}

	if err = envconfig.Process(cfg.prefix, &result); err != nil {
		return result, fmt.Errorf("failed to process envconfig: %w", err)
	}

	if len(cfg.filePath) > 0 {
		var file *os.File

		file, err = os.Open(cfg.filePath)
		if err != nil {
			return result, fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()

		if err = yaml.NewDecoder(file).Decode(&result); err != nil {
			return result, fmt.Errorf("failed to decode yaml: %w", err)
		}
	}

	return result, nil
}
