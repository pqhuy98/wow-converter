package config

import "os"

// IsDev reports local development mode (NODE_ENV=development).
func IsDev() bool {
	return os.Getenv("NODE_ENV") == "development"
}
