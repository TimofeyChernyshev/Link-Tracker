package receiver

import (
	context "context"
	"errors"
	"testing"

	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMultiReceiver_StartAllSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()

	receiver1 := NewMockReceiver(ctrl)
	receiver2 := NewMockReceiver(ctrl)
	receiver3 := NewMockReceiver(ctrl)

	receiver1.EXPECT().Start(ctx).Return(nil)
	receiver2.EXPECT().Start(ctx).Return(nil)
	receiver3.EXPECT().Start(ctx).Return(nil)

	multi := NewMultiReceiver([]Receiver{receiver1, receiver2, receiver3})

	err := multi.Start(ctx)

	require.NoError(t, err)
}

func TestMultiReceiver_OneFails_OthersSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()

	receiver1 := NewMockReceiver(ctrl)
	receiver2 := NewMockReceiver(ctrl)
	receiver3 := NewMockReceiver(ctrl)

	receiver1.EXPECT().Start(ctx).Return(errors.New("123"))
	receiver2.EXPECT().Start(ctx).Return(nil)
	receiver3.EXPECT().Start(ctx).Return(nil)

	multi := NewMultiReceiver([]Receiver{receiver1, receiver2, receiver3})

	err := multi.Start(ctx)

	require.NoError(t, err, "should succeed because at least one receiver works")
}

func TestMultiReceiver_AllFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()

	receiver1 := NewMockReceiver(ctrl)
	receiver2 := NewMockReceiver(ctrl)
	receiver3 := NewMockReceiver(ctrl)

	receiver1.EXPECT().Start(ctx).Return(errors.New("123"))
	receiver2.EXPECT().Start(ctx).Return(errors.New("321"))
	receiver3.EXPECT().Start(ctx).Return(errors.New("213"))

	multi := NewMultiReceiver([]Receiver{receiver1, receiver2, receiver3})

	err := multi.Start(ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "all receivers failed to start")
}

func TestMultiReceiver_EmptyReceivers(t *testing.T) {
	multi := NewMultiReceiver([]Receiver{})

	err := multi.Start(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "all receivers failed to start")
}

func TestMultiReceiver_Shutdown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	receiver1 := NewMockReceiver(ctrl)
	receiver2 := NewMockReceiver(ctrl)

	receiver1.EXPECT().Start(ctx).Return(nil)
	receiver2.EXPECT().Start(ctx).Return(nil)
	receiver1.EXPECT().Shutdown(ctx).Return(nil)
	receiver2.EXPECT().Shutdown(ctx).Return(nil)

	multi := NewMultiReceiver([]Receiver{receiver1, receiver2})

	err := multi.Start(ctx)
	require.NoError(t, err)

	err = multi.Shutdown(ctx)
	require.NoError(t, err)
}
