package storage

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
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

func TestGetAllLinks(t *testing.T) {
	s := New()
	s.AddLink(1, "123", []string{"tag"})
	s.AddLink(1, "256", []string{"tag"})
	s.AddLink(2, "123", []string{"tag"})
	s.AddLink(5, "678", []string{"tag"})

	expectedLinks := []domain.Link{
		{URL: "123", Tags: []string{"tag"}},
		{URL: "256", Tags: []string{"tag"}},
		{URL: "678", Tags: []string{"tag"}},
	}

	links := s.GetAllLinks()

	require.Len(t, links, 3)

	assert.Equal(t, expectedLinks[0].URL, links[0].URL)
	assert.Equal(t, expectedLinks[1].URL, links[1].URL)
	assert.Equal(t, expectedLinks[2].URL, links[2].URL)
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
