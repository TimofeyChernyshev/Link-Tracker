package inmemory

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
			assert.NoError(t, err)
		}()
	}

	wg.Wait()

	links := s.GetLinks(chatID)
	assert.Len(t, links, n)
}

func TestAddLink_Duplicate(t *testing.T) {
	s := New()
	chatID := int64(1)

	_, err := s.AddLink(chatID, "url1", nil)
	require.NoError(t, err)

	_, err = s.AddLink(chatID, "url1", nil)
	require.Error(t, err)
	assert.Equal(t, "link already tracked", err.Error())
}

func TestRemoveLink(t *testing.T) {
	s := New()
	chatID := int64(1)

	const n = 10

	for i := range n {
		url := fmt.Sprintf("url-%d", i)
		_, err := s.AddLink(chatID, url, nil)
		require.NoError(t, err, "failed to add link %s", url)
	}

	wg := sync.WaitGroup{}
	wg.Add(n)

	for i := range n {
		go func(i int) {
			defer wg.Done()
			url := fmt.Sprintf("url-%d", i)
			_, err := s.RemoveLink(chatID, url)
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()

	links := s.GetLinks(chatID)
	assert.Empty(t, links)
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

func TestGetAllLinks(t *testing.T) {
	s := New()
	s.AddLink(1, "123", []string{"tag"})
	s.AddLink(1, "256", []string{"tag"})
	s.AddLink(2, "123", []string{"tag"})
	s.AddLink(5, "678", []string{"tag"})

	expectedLinks := []string{"123", "256", "678"}

	links := s.GetAllLinks()

	require.Len(t, links, 3)

	assert.Contains(t, expectedLinks, links[0].URL)
	assert.Contains(t, expectedLinks, links[1].URL)
	assert.Contains(t, expectedLinks, links[2].URL)
}

func TestGetSubscribers(t *testing.T) {
	s := New()
	s.AddLink(1, "123", []string{"tag"})
	s.AddLink(1, "256", []string{"tag"})
	s.AddLink(2, "123", []string{"tag"})
	s.AddLink(5, "678", []string{"tag"})

	expectedSubs := []int64{1, 2}

	subs := s.GetSubscribers("123")

	assert.ElementsMatch(t, expectedSubs, subs)
}
