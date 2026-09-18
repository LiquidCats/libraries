package cfg_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LiquidCats/libraries/cfg"
)

type dbCfg struct {
	Host string `yaml:"host" default:"localhost"`
}

type testCfg struct {
	Port   int    `yaml:"port" default:"8080"`
	APIKey string `yaml:"api_key" required:"true"`
	DB     *dbCfg `yaml:"db"`
}

func writeYAML(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "cfg.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	return path
}

func TestLoad_DefaultSurvivesYAMLOverlay(t *testing.T) {
	t.Setenv("APIKEY", "from-env")

	path := writeYAML(t, "db:\n  host: db.internal\n")

	got, err := cfg.Load[testCfg](cfg.WithFilePath(path))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if got.Port != 8080 {
		t.Errorf("Port = %d, want default 8080", got.Port)
	}
	if got.DB.Host != "db.internal" {
		t.Errorf("DB.Host = %q, want yaml value db.internal", got.DB.Host)
	}
}

func TestLoad_YAMLOverridesDefault(t *testing.T) {
	t.Setenv("APIKEY", "from-env")

	path := writeYAML(t, "port: 9090\n")

	got, err := cfg.Load[testCfg](cfg.WithFilePath(path))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if got.Port != 9090 {
		t.Errorf("Port = %d, want yaml value 9090", got.Port)
	}
}

func TestLoad_RequiredMustComeFromEnv_YAMLDoesNotSatisfyIt(t *testing.T) {
	path := writeYAML(t, "api_key: from-yaml\n")

	_, err := cfg.Load[testCfg](cfg.WithFilePath(path))
	if err == nil {
		t.Fatal("expected error: required api_key is not set via env, YAML must not satisfy it")
	}
}

func TestLoad_RequiredSetViaEnv_Succeeds(t *testing.T) {
	t.Setenv("APIKEY", "from-env")

	path := writeYAML(t, "port: 9090\n")

	got, err := cfg.Load[testCfg](cfg.WithFilePath(path))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if got.APIKey != "from-env" {
		t.Errorf("APIKey = %q, want %q", got.APIKey, "from-env")
	}
}
