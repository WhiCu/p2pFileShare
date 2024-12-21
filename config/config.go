package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// init loads the .env file in the config directory into the environment, if one exists.
// This should be called automatically when the package is initialized.
func init() {
	if err := godotenv.Load("./config/.env"); err != nil {
		log.Print("No .env file found ")
	}
}

// Get retrieves the value of the environment variable named by the key.
// It returns the value and a boolean indicating whether the key was found.
func Get(key string) (string, bool) {
	return os.LookupEnv(key)
}

// MustGet retrieves the value of the environment variable named by the key.
// If the key is not found, it will panic with a message indicating the missing key.
func MustGet(key string) string {
	if value, ok := os.LookupEnv(key); !ok {
		panic("Missing environment variable: " + key)
	} else {
		return value
	}
}

func DefaultGet(key string, defaulValue string) string {
	if value, ok := os.LookupEnv(key); !ok {
		return defaulValue
	} else {
		return value
	}
}
