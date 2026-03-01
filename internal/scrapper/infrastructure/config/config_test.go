package config

import (
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

	err = os.Chdir(tempDir)
	require.NoError(t, err)
	defer os.Chdir(originalDir)

	content := []byte("PORT=8081\nBOT_BASE_URL=http://localhost:8080\n")
	err = os.WriteFile(".env", content, 0644)
	require.NoError(t, err)
	_ = godotenv.Load()

	cfg, err := Load()

	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "8081", cfg.ScrapperPort)
	assert.Equal(t, "http://localhost:8080", cfg.BotBaseURL)
}

func TestLoad_EnvFileNotFound(t *testing.T) {
	tempDir := t.TempDir()

	originalDir, err := os.Getwd()
	require.NoError(t, err)

	err = os.Chdir(tempDir)
	require.NoError(t, err)
	defer os.Chdir(originalDir)

	os.Remove(".env")

	os.Unsetenv("PORT")

	cfg, err := Load()

	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.ErrorContains(t, err, "port not found")
}
