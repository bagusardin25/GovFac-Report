package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	content := "# comment\n\nGOVFAC_TEST_A=plain\nexport GOVFAC_TEST_B=\"quoted value\"\nGOVFAC_TEST_C=from-file\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	// t.Setenv registers cleanup; we then unset A and B so the file can fill them.
	t.Setenv("GOVFAC_TEST_A", "")
	t.Setenv("GOVFAC_TEST_B", "")
	t.Setenv("GOVFAC_TEST_C", "from-env")
	os.Unsetenv("GOVFAC_TEST_A")
	os.Unsetenv("GOVFAC_TEST_B")

	if err := loadDotEnv(path); err != nil {
		t.Fatalf("loadDotEnv: %v", err)
	}

	want := map[string]string{
		"GOVFAC_TEST_A": "plain",
		"GOVFAC_TEST_B": "quoted value",
		"GOVFAC_TEST_C": "from-env",
	}
	for k, v := range want {
		if got := os.Getenv(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}

func TestLoadDotEnv_MissingFile(t *testing.T) {
	if err := loadDotEnv(filepath.Join(t.TempDir(), "missing.env")); err != nil {
		t.Fatalf("missing .env should not error: %v", err)
	}
}
