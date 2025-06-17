package thoth_test

import (
	"os"
	"testing"

	"github.com/TecharoHQ/anubis/internal/thoth"
	"github.com/TecharoHQ/anubis/internal/thoth/thothmock"
	"github.com/joho/godotenv"
)

func loadSecrets(t testing.TB) *thoth.Client {
	t.Helper()

	if err := godotenv.Load(); err != nil {
		t.Log("using mock thoth")
		result := &thoth.Client{}
		result.WithIPToASNService(thothmock.MockIpToASNService())
		return result
	}

	cli, err := thoth.New(t.Context(), os.Getenv("THOTH_URL"), os.Getenv("THOTH_API_KEY"), false)
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
