package data

import (
	"os"
	"testing"
)

func TestGetEnvOrEmpty(t *testing.T) {
	os.Unsetenv("BLADES_TEST_NONEXIST")
	if got := getEnvOr("BLADES_TEST_NONEXIST", "fallback"); got != "fallback" {
		t.Errorf("getEnvOr unset = %q, want fallback", got)
	}
}

func TestGetEnvOrTrim(t *testing.T) {
	t.Setenv("BLADES_TEST_VAR", "  trimmed  ")
	if got := getEnvOr("BLADES_TEST_VAR", "default"); got != "trimmed" {
		t.Errorf("getEnvOr with spaces = %q, want trimmed", got)
	}
}

func TestGetEnvOrEmptyValue(t *testing.T) {
	t.Setenv("BLADES_TEST_EMPTY", "")
	if got := getEnvOr("BLADES_TEST_EMPTY", "fallback"); got != "fallback" {
		t.Errorf("getEnvOr empty = %q, want fallback", got)
	}
}

func TestServiceURLDefaults(t *testing.T) {
	os.Unsetenv("BLADES_STOCK_STATIC_DATA_URL")
	os.Unsetenv("BLADES_STOCK_ADATA_URL")
	os.Unsetenv("BLADES_STOCK_AKSHARE_URL")
	os.Unsetenv("BLADES_STOCK_BAOSTOCK_URL")
	// Defaults are the package-level constants
	if got := StaticDataServiceURL(); got != defaultStaticDataServiceURL {
		t.Errorf("StaticDataServiceURL default = %q, want %q", got, defaultStaticDataServiceURL)
	}
	if got := ADataServiceURL(); got != defaultADataServiceURL {
		t.Errorf("ADataServiceURL default = %q, want %q", got, defaultADataServiceURL)
	}
	if got := AkshareServiceURL(); got != defaultAkshareServiceURL {
		t.Errorf("AkshareServiceURL default = %q, want %q", got, defaultAkshareServiceURL)
	}
	if got := BaostockServiceURL(); got != defaultBaostockServiceURL {
		t.Errorf("BaostockServiceURL default = %q, want %q", got, defaultBaostockServiceURL)
	}
}

func TestServiceURLsOverrideViaEnv(t *testing.T) {
	t.Setenv("BLADES_STOCK_STATIC_DATA_URL", "http://custom-static:9000")
	t.Setenv("BLADES_STOCK_ADATA_URL", "http://custom-adata:9001")
	t.Setenv("BLADES_STOCK_AKSHARE_URL", "http://custom-akshare:9002")
	t.Setenv("BLADES_STOCK_BAOSTOCK_URL", "http://custom-baostock:9003")
	if got := StaticDataServiceURL(); got != "http://custom-static:9000" {
		t.Errorf("StaticDataServiceURL override = %q", got)
	}
	if got := ADataServiceURL(); got != "http://custom-adata:9001" {
		t.Errorf("ADataServiceURL override = %q", got)
	}
	if got := AkshareServiceURL(); got != "http://custom-akshare:9002" {
		t.Errorf("AkshareServiceURL override = %q", got)
	}
	if got := BaostockServiceURL(); got != "http://custom-baostock:9003" {
		t.Errorf("BaostockServiceURL override = %q", got)
	}
}
