package thoth

import (
	"os"
	"testing"

	"github.com/joho/godotenv"
)

func loadSecrets(t *testing.T) *Client {
	if err := godotenv.Load(); err != nil {
		t.Skip(".env not defined, can't load thoth secrets")
	}

	cli, err := New(t.Context(), os.Getenv("THOTH_URL"), os.Getenv("THOTH_API_KEY"))
	if err != nil {
		t.Fatal(err)
	}

	return cli
}

func TestNew(t *testing.T) {
	cli := loadSecrets(t)

	if err := cli.Close(); err != nil {
		t.Fatal(err)
	}
}
