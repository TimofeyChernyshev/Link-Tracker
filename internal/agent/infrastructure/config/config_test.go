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

	content := []byte("KAFKA_BROKERS=x,23\nAGENT_KAFKA_DLQ_TOPIC=1\nAGENT_KAFKA_CONSUMER_TOPIC=1\nAGENT_KAFKA_GROUP_ID=1\nAGENT_KAFKA_NOTIFIER_TOPIC=1")
	err = os.WriteFile(".env", content, 0644)
	require.NoError(t, err)
	_ = godotenv.Load()

	cfg, err := Load()

	fmt.Println(cfg)

	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, []string(nil), cfg.FilterStopWords)
}
