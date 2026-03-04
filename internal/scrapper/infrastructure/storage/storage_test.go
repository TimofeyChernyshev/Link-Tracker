package storage

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterAndExists(t *testing.T) {
	s := New()

	assert.False(t, s.ChatExists(1))

	s.RegisterChat(1)

	assert.True(t, s.ChatExists(1))
}

func TestDeleteChat(t *testing.T) {
	s := New()

	s.RegisterChat(1)
	s.DeleteChat(1)

	assert.False(t, s.ChatExists(1))
}

func TestAddLink(t *testing.T) {
	s := New()
	chatID := int64(1)

	s.RegisterChat(chatID)

	const n = 10

	wg := sync.WaitGroup{}
	wg.Add(n)

	for i := range n {
		go func() {
			defer wg.Done()

			url := fmt.Sprintf("url-%d", i)
			_, err := s.AddLink(chatID, url, []string{"tag"})
			require.NoError(t, err)
		}()
	}

	wg.Wait()

	links := s.GetLinks(chatID)
	assert.Len(t, links, n)
}

func TestAddLink_Duplicate(t *testing.T) {
	s := New()
	chatID := int64(1)
	s.RegisterChat(chatID)

	_, err := s.AddLink(chatID, "url1", nil)
	require.NoError(t, err)

	_, err = s.AddLink(chatID, "url1", nil)
	require.Error(t, err)
	assert.Equal(t, "link already tracked", err.Error())
}

func TestRemoveLink(t *testing.T) {
	s := New()
	chatID := int64(1)
	s.RegisterChat(chatID)

	const n = 10

	wg := sync.WaitGroup{}
	wg.Add(n * 2)

	for i := range n {
		go func() {
			defer wg.Done()
			url := fmt.Sprintf("url-%d", i)
			s.AddLink(chatID, url, nil)
		}()

		go func() {
			defer wg.Done()
			url := fmt.Sprintf("url-%d", i)
			s.RemoveLink(chatID, url)
		}()
	}

	wg.Wait()
}

func TestRemoveLink_NotFound(t *testing.T) {
	t.Parallel()
	s := New()
	chatID := int64(1)
	s.RegisterChat(chatID)

	_, err := s.RemoveLink(chatID, "url1")
	require.Error(t, err)
	assert.Equal(t, "link not found", err.Error())
}
