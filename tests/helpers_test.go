package tests

import (
	"strings"
)

func hasEnv(envs []string, key, value string) bool {
	want := key + "=" + value
	for _, e := range envs {
		if e == want {
			return true
		}
	}
	return false
}

func contains(slice []string, target string) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
}

func containsAny(slice []string, substr string) bool {
	for _, s := range slice {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}
