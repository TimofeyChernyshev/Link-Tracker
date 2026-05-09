package scrappernotifier

import (
	"context"
	"errors"
	"testing"

	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

func TestFallbackNotifier_SendUpdate_FirstSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockNotifier1 := NewMockNotifier(ctrl)
	mockNotifier2 := NewMockNotifier(ctrl)

	upd := domain.LinkUpdate{ID: 1, URL: "https://test.com"}

	mockNotifier1.EXPECT().SendUpdate(gomock.Any(), upd).Return(nil).Times(1)
	mockNotifier2.EXPECT().SendUpdate(gomock.Any(), gomock.Any()).Times(0)

	fallback := NewFallbackNotifier([]Notifier{mockNotifier1, mockNotifier2})

	err := fallback.SendUpdate(context.Background(), upd)

	assert.NoError(t, err)
}

func TestFallbackNotifier_SendUpdate_FirstFails_SecondSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockNotifier1 := NewMockNotifier(ctrl)
	mockNotifier2 := NewMockNotifier(ctrl)

	upd := domain.LinkUpdate{ID: 1, URL: "https://test.com"}
	firstErr := errors.New("primary notifier unavailable")

	mockNotifier1.EXPECT().SendUpdate(gomock.Any(), upd).Return(firstErr).Times(1)
	mockNotifier2.EXPECT().SendUpdate(gomock.Any(), upd).Return(nil).Times(1)

	fallback := NewFallbackNotifier([]Notifier{mockNotifier1, mockNotifier2})

	err := fallback.SendUpdate(context.Background(), upd)

	assert.NoError(t, err)
}

func TestFallbackNotifier_SendUpdate_AllFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockNotifier1 := NewMockNotifier(ctrl)
	mockNotifier2 := NewMockNotifier(ctrl)
	mockNotifier3 := NewMockNotifier(ctrl)

	upd := domain.LinkUpdate{ID: 1, URL: "https://test.com"}
	err1 := errors.New("first error")
	err2 := errors.New("second error")
	err3 := errors.New("third error")

	mockNotifier1.EXPECT().SendUpdate(gomock.Any(), upd).Return(err1).Times(1)
	mockNotifier2.EXPECT().SendUpdate(gomock.Any(), upd).Return(err2).Times(1)
	mockNotifier3.EXPECT().SendUpdate(gomock.Any(), upd).Return(err3).Times(1)

	fallback := NewFallbackNotifier([]Notifier{mockNotifier1, mockNotifier2, mockNotifier3})

	err := fallback.SendUpdate(context.Background(), upd)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "all notifiers failed")
	assert.Contains(t, err.Error(), "third error")
}

func TestFallbackNotifier_SendUpdate_EmptyNotifiers(t *testing.T) {
	fallback := NewFallbackNotifier([]Notifier{})

	err := fallback.SendUpdate(context.Background(), domain.LinkUpdate{ID: 1})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "all notifiers failed")
}

func TestFallbackNotifier_Close_AllSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockNotifier1 := NewMockNotifier(ctrl)
	mockNotifier2 := NewMockNotifier(ctrl)

	mockNotifier1.EXPECT().Close().Return(nil).Times(1)
	mockNotifier2.EXPECT().Close().Return(nil).Times(1)

	fallback := NewFallbackNotifier([]Notifier{mockNotifier1, mockNotifier2})

	err := fallback.Close()

	assert.NoError(t, err)
}

func TestFallbackNotifier_Close_WithErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockNotifier1 := NewMockNotifier(ctrl)
	mockNotifier2 := NewMockNotifier(ctrl)
	mockNotifier3 := NewMockNotifier(ctrl)

	err1 := errors.New("close error 1")
	err3 := errors.New("close error 3")

	mockNotifier1.EXPECT().Close().Return(err1).Times(1)
	mockNotifier2.EXPECT().Close().Return(nil).Times(1)
	mockNotifier3.EXPECT().Close().Return(err3).Times(1)

	fallback := NewFallbackNotifier([]Notifier{mockNotifier1, mockNotifier2, mockNotifier3})

	err := fallback.Close()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), err1.Error())
	assert.Contains(t, err.Error(), err3.Error())
}

func TestFallbackNotifier_Close_EmptyNotifiers(t *testing.T) {
	fallback := NewFallbackNotifier([]Notifier{})

	err := fallback.Close()

	assert.NoError(t, err)
}
