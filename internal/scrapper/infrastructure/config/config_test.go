package config

import (
	"fmt"
	"os"
	"testing"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Success(t *testing.T) {
	tempDir := t.TempDir()

	originalDir, err := os.Getwd()
	require.NoError(t, err)

	t.Chdir(tempDir)
	require.NoError(t, err)
	defer t.Chdir(originalDir)

	content := []byte("SCRAPPER_PORT=8081\nBOT_BASE_URL=http://localhost:8080\n" +
		"ACCESS_TYPE=1\nDB_USER=1\nDB_PASSWORD=1\nDB_HOST=1\nDB_PORT=1\nDB_NAME=1" +
		"\nGITHUB_BASE_URL=http\nSTACK_BASE_URL=http")
	err = os.WriteFile(".env", content, 0644)
	require.NoError(t, err)
	_ = godotenv.Load()

	cfg, err := Load()

	fmt.Println(cfg)

	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "8081", cfg.ScrapperPort)
	assert.Equal(t, "http://localhost:8080", cfg.BotBaseURL)
}

func TestLoad_EnvFileNotFound(t *testing.T) {
	tempDir := t.TempDir()

	originalDir, err := os.Getwd()
	require.NoError(t, err)

	t.Chdir(tempDir)
	require.NoError(t, err)
	defer t.Chdir(originalDir)

	os.Remove(".env")

	os.Unsetenv("SCRAPPER_PORT")

	cfg, err := Load()

	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.ErrorContains(t, err, "is not set")
}
