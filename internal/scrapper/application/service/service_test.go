package service

import (
	"testing"

	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

func TestService_AddLink_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := NewMockStorage(ctrl)
	svc := New(st)

	chatID := int64(1)
	url := "https://example.com"
	tags := []string{"go"}

	expected := domain.Link{
		URL:  url,
		Tags: tags,
	}

	st.EXPECT().ChatExists(chatID).Return(true)
	st.EXPECT().AddLink(chatID, url, tags).Return(expected, nil)

	link, err := svc.AddLink(chatID, url, tags)

	require.NoError(t, err)
	assert.Equal(t, expected, link)
}

func TestService_AddLink_ChatNotExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := NewMockStorage(ctrl)
	svc := New(st)

	st.EXPECT().ChatExists(int64(1)).Return(false)

	link, err := svc.AddLink(1, "url", nil)

	require.Error(t, err)
	require.ErrorIs(t, err, errChatInstRegistered)
	assert.Equal(t, domain.Link{}, link)
}

func TestService_RemoveLink_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := NewMockStorage(ctrl)
	svc := New(st)

	expected := domain.Link{URL: "https://example.com"}

	st.EXPECT().ChatExists(int64(1)).Return(true)
	st.EXPECT().RemoveLink(int64(1), expected.URL).Return(expected, nil)

	link, err := svc.RemoveLink(1, expected.URL)

	require.NoError(t, err)
	assert.Equal(t, expected, link)
}

func TestService_RemoveLink_ChatNotExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := NewMockStorage(ctrl)
	svc := New(st)

	st.EXPECT().ChatExists(int64(1)).Return(false)

	_, err := svc.RemoveLink(1, "url")

	require.Error(t, err)
	assert.ErrorIs(t, err, errChatInstRegistered)
}

func TestService_GetLinks_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := NewMockStorage(ctrl)
	svc := New(st)

	expected := []domain.Link{
		{URL: "a"},
		{URL: "b"},
	}

	st.EXPECT().ChatExists(int64(1)).Return(true)
	st.EXPECT().GetLinks(int64(1)).Return(expected)

	links, err := svc.GetLinks(1)

	require.NoError(t, err)
	assert.Equal(t, expected, links)
}

func TestService_GetLinks_ChatNotExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := NewMockStorage(ctrl)
	svc := New(st)

	st.EXPECT().ChatExists(int64(1)).Return(false)

	links, err := svc.GetLinks(1)

	require.Error(t, err)
	require.ErrorIs(t, err, errChatInstRegistered)
	assert.Nil(t, links)
}

func TestService_RegisterChat_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := NewMockStorage(ctrl)
	svc := New(st)

	st.EXPECT().ChatExists(int64(1)).Return(false)
	st.EXPECT().RegisterChat(int64(1))

	err := svc.RegisterChat(1)

	require.NoError(t, err)
}

func TestService_RegisterChat_AlreadyExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := NewMockStorage(ctrl)
	svc := New(st)

	st.EXPECT().ChatExists(int64(1)).Return(true)

	err := svc.RegisterChat(1)

	require.Error(t, err)
	assert.Equal(t, "chat already registered", err.Error())
}

func TestService_DeleteChat_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := NewMockStorage(ctrl)
	svc := New(st)

	st.EXPECT().ChatExists(int64(1)).Return(true)
	st.EXPECT().DeleteChat(int64(1))

	err := svc.DeleteChat(1)

	require.NoError(t, err)
}

func TestService_DeleteChat_NotExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := NewMockStorage(ctrl)
	svc := New(st)

	st.EXPECT().ChatExists(int64(1)).Return(false)

	err := svc.DeleteChat(1)

	require.Error(t, err)
	assert.ErrorIs(t, err, errChatInstRegistered)
}
