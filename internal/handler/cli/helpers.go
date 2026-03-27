package cli

import (
	"fmt"
	"strings"
)

// buildFlagDefMap builds a fast lookup map for command flag definitions.
func buildFlagDefMap(cmd *CommandDefinition) map[string]FlagDefinition {
	flagDefs := make(map[string]FlagDefinition, len(cmd.Flags))
	for _, f := range cmd.Flags {
		flagDefs[f.Name] = f
	}
	return flagDefs
}

// isFlagToken reports whether the token starts with "--".
func isFlagToken(s string) bool {
	return strings.HasPrefix(s, "--")
}

// parseLongFlag parses "--name value" or "--name=value" format.
func parseLongFlag(arg string) (name, value string) {
	arg = strings.TrimPrefix(arg, "--")
	if idx := strings.Index(arg, "="); idx >= 0 {
		return arg[:idx], arg[idx+1:]
	}
	return arg, ""
}

// normalizeBoolFlag normalizes bool-like values to "true"/"false".
func normalizeBoolFlag(raw string) (string, error) {
	parsed, ok := parseBool(raw)
	if !ok {
		return "", fmt.Errorf("must be one of: true, false, 1, 0, yes, no")
	}
	if parsed {
		return "true", nil
	}
	return "false", nil
}

