package v1

import (
	"context"
	"testing"

	mqttLib "github.com/at-wat/mqtt-go"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockMQTTLibClient struct {
	mock.Mock
}

func (m *mockMQTTLibClient) Connect(ctx context.Context, clientID string, opts ...mqttLib.ConnectOption) (sessionPresent bool, err error) {
	args := m.Called(ctx, clientID, opts)

	return args.Bool(0), args.Error(1)
}

func (m *mockMQTTLibClient) Disconnect(ctx context.Context) error {
	args := m.Called(ctx)

	return args.Error(0)
}

func (m *mockMQTTLibClient) Publish(ctx context.Context, message *mqttLib.Message) error {
	args := m.Called(ctx, message)

	return args.Error(0)
}

func (m *mockMQTTLibClient) Subscribe(ctx context.Context, subs ...mqttLib.Subscription) ([]mqttLib.Subscription, error) {
	args := m.Called(ctx, subs)

	return nil, args.Error(0)
}

func (m *mockMQTTLibClient) Unsubscribe(ctx context.Context, subs ...string) error {
	args := m.Called(ctx, subs)

	return args.Error(0)
}

func (m *mockMQTTLibClient) Ping(ctx context.Context) error {
	args := m.Called(ctx)

	return args.Error(0)
}

func (m *mockMQTTLibClient) Handle(handler mqttLib.Handler) {
	m.Called(handler)
}

func TestMqttClientPublish(t *testing.T) {
	testCases := []struct {
		name    string
		topic   string
		payload []byte
		retain  bool
		qos     int
	}{
		{
			name:    "Simple publish",
			topic:   "test",
			payload: []byte("test"),
			retain:  true,
			qos:     1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mc := new(mockMQTTLibClient)
			c := &mqttClient{
				client: mc,
			}

			var expectedQoS mqttLib.QoS
			switch tc.qos {
			case 0:
				expectedQoS = mqttLib.QoS0
			case 1:
				expectedQoS = mqttLib.QoS1
			case 2:
				expectedQoS = mqttLib.QoS2
			default:
				require.Fail(t, "invalid QoS level")
			}

			ctx := context.Background()
			mc.On("Publish", ctx, &mqttLib.Message{
				Topic:   tc.topic,
				Payload: tc.payload,
				QoS:     expectedQoS,
				Retain:  tc.retain,
			}).Return(nil)

			err := c.Publish(ctx, message{
				topic:   tc.topic,
				payload: tc.payload,
				retain:  tc.retain,
				qos:     tc.qos,
			})

			require.NoError(t, err)
			mc.AssertExpectations(t)
		})
	}
}
