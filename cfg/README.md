# cfg

Loads a config struct from environment variables and, optionally, a YAML file.
Env handling is [`kelseyhightower/envconfig`](https://github.com/kelseyhightower/envconfig),
YAML is [`goccy/go-yaml`](https://github.com/goccy/go-yaml).

```sh
go get github.com/LiquidCats/libraries/cfg
```

```go
type Config struct {
	Port   int    `yaml:"port" default:"8080"`
	APIKey string `yaml:"api_key" required:"true" envconfig:"API_KEY"`
	DB     struct {
		Host string `yaml:"host" default:"localhost"`
	} `yaml:"db"`
}

conf, err := cfg.Load[Config](
	cfg.WithPrefix("APP"),             // APP_PORT, APP_API_KEY, APP_DB_HOST
	cfg.WithFilePath("config.yaml"),   // optional
)
```

## Order of precedence

1. `envconfig` runs first: environment variables, then `default:"..."` tags for
   anything not set.
2. If `WithFilePath` is given, the YAML file is decoded on top. Only keys present
   in the file are written, so defaults and env values survive for everything
   the file does not mention — including partially specified nested structs.

**The file wins over the environment** for any key it sets.

## Tags

- `default:"x"` works as in `envconfig`, and still applies when you load from
  YAML (it is applied before the file is read).
- `required:"true"` is enforced by `envconfig` against the **environment only**.
  A required field supplied solely by the YAML file still fails with
  `required key ... missing value`. Give such a field a `default` or set it via
  an env var.
- Env var names come from the field name (`APIKey` -> `APIKEY`) unless you set
  `envconfig:"NAME"` or `split_words:"true"`; see the `envconfig` docs.

## Errors

`Load` wraps failures with `%w`: `failed to process envconfig`,
`failed to open file`, `failed to decode yaml`.
