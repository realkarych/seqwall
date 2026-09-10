package seqwall

import "runtime"

const (
	CurrentMigrationPlaceholder = "{current_migration}"
	CurrentMigrationEnv         = "SEQWALL_CURRENT_MIGRATION"
)

func supportsLegacyPlaceholder(migration string) bool {
	for i := 0; i < len(migration); i++ {
		c := migration[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
			c == '_' || c == '-' || c == '.' || c == '/' {
			continue
		}
		if runtime.GOOS == "windows" && (c == '\\' || c == ':') {
			continue
		}
		return false
	}
	return true
}
