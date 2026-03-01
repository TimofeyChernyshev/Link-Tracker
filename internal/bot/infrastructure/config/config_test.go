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

	t.Chdir(tempDir)
	defer t.Chdir(originalDir)

	content := []byte("TELEGRAM_TOKEN=test_token_12345\nPORT=8080\nSCRAPPER_BASE_URL=http://123\n")
	err = os.WriteFile(".env", content, 0644)
	require.NoError(t, err)
	_ = godotenv.Load()

	cfg, err := Load()

	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "test_token_12345", cfg.TelegramToken)
}

func TestLoad_EnvFileNotFound(t *testing.T) {
	tempDir := t.TempDir()

	originalDir, err := os.Getwd()
	require.NoError(t, err)

	t.Chdir(tempDir)
	defer t.Chdir(originalDir)

	os.Remove(".env")

	os.Unsetenv("TELEGRAM_TOKEN")

	cfg, err := Load()

	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.ErrorContains(t, err, "token not found")
}

func TestLoad_TokenNotSet(t *testing.T) {
	tempDir := t.TempDir()

	originalDir, err := os.Getwd()
	require.NoError(t, err)

	t.Chdir(tempDir)
	defer t.Chdir(originalDir)

	content := []byte("SOME_OTHER_VAR=value\nANOTHER_VAR=123\n")
	err = os.WriteFile(".env", content, 0644)
	require.NoError(t, err)

	os.Unsetenv("TELEGRAM_TOKEN")
	_ = godotenv.Load()

	cfg, err := Load()

	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Equal(t, "token not found", err.Error())
}

func TestLoad_EmptyToken(t *testing.T) {
	tempDir := t.TempDir()

	originalDir, err := os.Getwd()
	require.NoError(t, err)

	t.Chdir(tempDir)
	defer t.Chdir(originalDir)

	content := []byte("TELEGRAM_TOKEN=\n")
	err = os.WriteFile(".env", content, 0644)
	require.NoError(t, err)
	_ = godotenv.Load()

	cfg, err := Load()

	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Equal(t, "token not found", err.Error())
}
