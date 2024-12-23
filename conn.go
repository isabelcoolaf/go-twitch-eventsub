package twitch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"nhooyr.io/websocket"
)

const (
	twitchWebsocketUrl = "wss://eventsub.wss.twitch.tv/ws"
)

var (
	ErrConnClosed   = fmt.Errorf("connection closed")
	ErrNilOnWelcome = fmt.Errorf("OnWelcome function was not set")

	messageTypeMap = map[string]func() any{
		"session_welcome":   zeroPtrGen[WelcomeMessage](),
		"session_keepalive": zeroPtrGen[KeepAliveMessage](),
		"notification":      zeroPtrGen[NotificationMessage](),
		"session_reconnect": zeroPtrGen[ReconnectMessage](),
		"revocation":        zeroPtrGen[RevokeMessage](),
	}
)

func zeroPtrGen[T any]() func() any {
	return func() any {
		return new(T)
	}
}

func callFunc[T any, C any](f func(T, C), v T, c C) {
	if f != nil {
		go f(v, c)
	}
}

type Client struct {
	Address   string
	ws        *websocket.Conn
	connected bool
	ctx       context.Context

	reconnecting bool
	reconnected  chan struct{}

	// Responses
	onError        func(err error)
	onWelcome      func(message WelcomeMessage, metadata MessageMetadata)
	onKeepAlive    func(message KeepAliveMessage, metadata MessageMetadata)
	onNotification func(message NotificationMessage, metadata MessageMetadata)
	onReconnect    func(message ReconnectMessage, metadata MessageMetadata)
	onRevoke       func(message RevokeMessage, metadata MessageMetadata)

	// Events
	onRawEvent                                              func(event string, metadata MessageMetadata, subscription PayloadSubscription)
	onEventChannelUpdate                                    func(event EventChannelUpdate, payloadContext PayloadContext)
	onEventChannelFollow                                    func(event EventChannelFollow, payloadContext PayloadContext)
	onEventChannelSubscribe                                 func(event EventChannelSubscribe, payloadContext PayloadContext)
	onEventChannelSubscriptionEnd                           func(event EventChannelSubscriptionEnd, payloadContext PayloadContext)
	onEventChannelSubscriptionGift                          func(event EventChannelSubscriptionGift, payloadContext PayloadContext)
	onEventChannelSubscriptionMessage                       func(event EventChannelSubscriptionMessage, payloadContext PayloadContext)
	onEventChannelCheer                                     func(event EventChannelCheer, payloadContext PayloadContext)
	onEventChannelRaid                                      func(event EventChannelRaid, payloadContext PayloadContext)
	onEventChannelBan                                       func(event EventChannelBan, payloadContext PayloadContext)
	onEventChannelUnban                                     func(event EventChannelUnban, payloadContext PayloadContext)
	onEventChannelModeratorAdd                              func(event EventChannelModeratorAdd, payloadContext PayloadContext)
	onEventChannelModeratorRemove                           func(event EventChannelModeratorRemove, payloadContext PayloadContext)
	onEventChannelVIPAdd                                    func(event EventChannelVIPAdd, payloadContext PayloadContext)
	onEventChannelVIPRemove                                 func(event EventChannelVIPRemove, payloadContext PayloadContext)
	onEventChannelChannelPointsCustomRewardAdd              func(event EventChannelChannelPointsCustomRewardAdd, payloadContext PayloadContext)
	onEventChannelChannelPointsCustomRewardUpdate           func(event EventChannelChannelPointsCustomRewardUpdate, payloadContext PayloadContext)
	onEventChannelChannelPointsCustomRewardRemove           func(event EventChannelChannelPointsCustomRewardRemove, payloadContext PayloadContext)
	onEventChannelChannelPointsCustomRewardRedemptionAdd    func(event EventChannelChannelPointsCustomRewardRedemptionAdd, payloadContext PayloadContext)
	onEventChannelChannelPointsCustomRewardRedemptionUpdate func(event EventChannelChannelPointsCustomRewardRedemptionUpdate, payloadContext PayloadContext)
	onEventChannelChannelPointsAutomaticRewardRedemptionAdd func(event EventChannelChannelPointsAutomaticRewardRedemptionAdd, payloadContext PayloadContext)
	onEventChannelPollBegin                                 func(event EventChannelPollBegin, payloadContext PayloadContext)
	onEventChannelPollProgress                              func(event EventChannelPollProgress, payloadContext PayloadContext)
	onEventChannelPollEnd                                   func(event EventChannelPollEnd, payloadContext PayloadContext)
	onEventChannelPredictionBegin                           func(event EventChannelPredictionBegin, payloadContext PayloadContext)
	onEventChannelPredictionProgress                        func(event EventChannelPredictionProgress, payloadContext PayloadContext)
	onEventChannelPredictionLock                            func(event EventChannelPredictionLock, payloadContext PayloadContext)
	onEventChannelPredictionEnd                             func(event EventChannelPredictionEnd, payloadContext PayloadContext)
	onEventDropEntitlementGrant                             func(event []EventDropEntitlementGrant, payloadContext PayloadContext)
	onEventExtensionBitsTransactionCreate                   func(event EventExtensionBitsTransactionCreate, payloadContext PayloadContext)
	onEventChannelGoalBegin                                 func(event EventChannelGoalBegin, payloadContext PayloadContext)
	onEventChannelGoalProgress                              func(event EventChannelGoalProgress, payloadContext PayloadContext)
	onEventChannelGoalEnd                                   func(event EventChannelGoalEnd, payloadContext PayloadContext)
	onEventChannelHypeTrainBegin                            func(event EventChannelHypeTrainBegin, payloadContext PayloadContext)
	onEventChannelHypeTrainProgress                         func(event EventChannelHypeTrainProgress, payloadContext PayloadContext)
	onEventChannelHypeTrainEnd                              func(event EventChannelHypeTrainEnd, payloadContext PayloadContext)
	onEventStreamOnline                                     func(event EventStreamOnline, payloadContext PayloadContext)
	onEventStreamOffline                                    func(event EventStreamOffline, payloadContext PayloadContext)
	onEventUserAuthorizationGrant                           func(event EventUserAuthorizationGrant, payloadContext PayloadContext)
	onEventUserAuthorizationRevoke                          func(event EventUserAuthorizationRevoke, payloadContext PayloadContext)
	onEventUserUpdate                                       func(event EventUserUpdate, payloadContext PayloadContext)
	onEventChannelCharityCampaignDonate                     func(event EventChannelCharityCampaignDonate, payloadContext PayloadContext)
	onEventChannelCharityCampaignProgress                   func(event EventChannelCharityCampaignProgress, payloadContext PayloadContext)
	onEventChannelCharityCampaignStart                      func(event EventChannelCharityCampaignStart, payloadContext PayloadContext)
	onEventChannelCharityCampaignStop                       func(event EventChannelCharityCampaignStop, payloadContext PayloadContext)
	onEventChannelShieldModeBegin                           func(event EventChannelShieldModeBegin, payloadContext PayloadContext)
	onEventChannelShieldModeEnd                             func(event EventChannelShieldModeEnd, payloadContext PayloadContext)
	onEventChannelShoutoutCreate                            func(event EventChannelShoutoutCreate, payloadContext PayloadContext)
	onEventChannelShoutoutReceive                           func(event EventChannelShoutoutReceive, payloadContext PayloadContext)
	onEventChannelModerate                                  func(event EventChannelModerate, payloadContext PayloadContext)
	onEventChannelAdBreakBegin                              func(event EventChannelAdBreakBegin, payloadContext PayloadContext)
	onEventChannelWarningAcknowledge                        func(event EventChannelWarningAcknowledge, payloadContext PayloadContext)
	onEventChannelWarningSend                               func(event EventChannelWarningSend, payloadContext PayloadContext)
	onEventChannelUnbanRequestCreate                        func(event EventChannelUnbanRequestCreate, payloadContext PayloadContext)
	onEventChannelUnbanRequestResolve                       func(event EventChannelUnbanRequestResolve, payloadContext PayloadContext)
	onEventAutomodMessageHold                               func(event EventAutomodMessageHold, payloadContext PayloadContext)
	onEventAutomodMessageUpdate                             func(event EventAutomodMessageUpdate, payloadContext PayloadContext)
	onEventAutomodSettingsUpdate                            func(event EventAutomodSettingsUpdate, payloadContext PayloadContext)
	onEventAutomodTermsUpdate                               func(event EventAutomodTermsUpdate, payloadContext PayloadContext)
	onEventChannelChatUserMessageHold                       func(event EventChannelChatUserMessageHold, payloadContext PayloadContext)
	onEventChannelChatUserMessageUpdate                     func(event EventChannelChatUserMessageUpdate, payloadContext PayloadContext)
	onEventChannelChatClear                                 func(event EventChannelChatClear, payloadContext PayloadContext)
	onEventChannelChatClearUserMessages                     func(event EventChannelChatClearUserMessages, payloadContext PayloadContext)
	onEventChannelChatMessage                               func(event EventChannelChatMessage, payloadContext PayloadContext)
	onEventChannelChatMessageDelete                         func(event EventChannelChatMessageDelete, payloadContext PayloadContext)
	onEventChannelChatNotification                          func(event EventChannelChatNotification, payloadContext PayloadContext)
	onEventChannelChatSettingsUpdate                        func(event EventChannelChatSettingsUpdate, payloadContext PayloadContext)
	onEventChannelSuspiciousUserMessage                     func(event EventChannelSuspiciousUserMessage, payloadContext PayloadContext)
	onEventChannelSuspiciousUserUpdate                      func(event EventChannelSuspiciousUserUpdate, payloadContext PayloadContext)
	onEventChannelSharedChatBegin                           func(event EventChannelSharedChatBegin, payloadContext PayloadContext)
	onEventChannelSharedChatUpdate                          func(event EventChannelSharedChatUpdate, payloadContext PayloadContext)
	onEventChannelSharedChatEnd                             func(event EventChannelSharedChatEnd, payloadContext PayloadContext)
	onEventUserWhisperMessage                               func(event EventUserWhisperMessage, payloadContext PayloadContext)
	onEventConduitShardDisabled                             func(event EventConduitShardDisabled, payloadContext PayloadContext)
}

func NewClient() *Client {
	return NewClientWithUrl(twitchWebsocketUrl)
}

func NewClientWithUrl(url string) *Client {
	return &Client{
		Address:     url,
		reconnected: make(chan struct{}),
		onError:     func(err error) { fmt.Printf("ERROR: %v\n", err) },
	}
}

func (c *Client) Connect() error {
	return c.ConnectWithContext(context.Background())
}

func (c *Client) ConnectWithContext(ctx context.Context) error {
	if c.onWelcome == nil {
		return ErrNilOnWelcome
	}

	c.ctx = ctx
	ws, err := c.dial()
	if err != nil {
		return err
	}
	c.ws = ws
	c.connected = true

	for {
		_, data, err := c.ws.Read(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}

			if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
				if c.reconnecting {
					c.reconnecting = false
					<-c.reconnected
					continue
				}
				return nil
			}

			return fmt.Errorf("could not read message: %w", err)
		}

		err = c.handleMessage(data)
		if err != nil {
			c.onError(err)
		}
	}
}

func (c *Client) Close() error {
	defer func() { c.ws = nil }()
	if !c.connected {
		return nil
	}
	c.connected = false

	err := c.ws.Close(websocket.StatusNormalClosure, "Stopping Connection")

	var closeError websocket.CloseError
	if err != nil && !errors.As(err, &closeError) {
		return fmt.Errorf("could not close websocket connection: %w", err)
	}
	return nil
}

func (c *Client) handleMessage(data []byte) error {
	metadata, err := parseBaseMessage(data)
	if err != nil {
		return err
	}

	messageType := metadata.MessageType
	genMessage, ok := messageTypeMap[messageType]
	if !ok {
		return fmt.Errorf("unknown message type %s: %s", messageType, string(data))
	}

	message := genMessage()
	err = json.Unmarshal(data, message)
	if err != nil {
		return fmt.Errorf("could not unmarshal message into %s: %w", messageType, err)
	}

	switch msg := message.(type) {
	case *WelcomeMessage:
		callFunc(c.onWelcome, *msg, metadata)
	case *KeepAliveMessage:
		callFunc(c.onKeepAlive, *msg, metadata)
	case *NotificationMessage:
		callFunc(c.onNotification, *msg, metadata)

		err = c.handleNotification(*msg)
		if err != nil {
			return fmt.Errorf("could not handle notification: %w", err)
		}
	case *ReconnectMessage:
		callFunc(c.onReconnect, *msg, metadata)

		err = c.reconnect(*msg)
		if err != nil {
			return fmt.Errorf("could not handle reconnect: %w", err)
		}
	case *RevokeMessage:
		callFunc(c.onRevoke, *msg, metadata)
	default:
		return fmt.Errorf("unhandled %T message: %v", msg, msg)
	}

	return nil
}

func (c *Client) reconnect(message ReconnectMessage) error {
	c.Address = message.Payload.Session.ReconnectUrl
	ws, err := c.dial()
	if err != nil {
		return fmt.Errorf("could not dial to reconnect")
	}

	go func() {
		_, data, err := ws.Read(c.ctx)
		if err != nil {
			c.onError(fmt.Errorf("reconnect failed: could not read reconnect websocket for welcome: %w", err))
		}

		metadata, err := parseBaseMessage(data)
		if err != nil {
			c.onError(fmt.Errorf("reconnect failed: could parse base message: %w", err))
		}

		if metadata.MessageType != "session_welcome" {
			c.onError(fmt.Errorf("reconnect failed: did not get a session_welcome message first: got message %s", metadata.MessageType))
			return
		}

		c.reconnecting = true
		c.ws.Close(websocket.StatusNormalClosure, "Stopping Connection")
		c.ws = ws
		c.reconnected <- struct{}{}
	}()

	return nil
}

func (c *Client) handleNotification(message NotificationMessage) error {
	data, err := message.Payload.Event.MarshalJSON()
	if err != nil {
		return fmt.Errorf("could not get event json: %w", err)
	}

	subscription := message.Payload.Subscription
	metadata, ok := subMetadata[subscription.Type]
	if !ok {
		return fmt.Errorf("unknown subscription type %s", subscription.Type)
	}

	if c.onRawEvent != nil {
		c.onRawEvent(string(data), message.Metadata, subscription)
	}

	var newEvent any
	if metadata.EventGen != nil {
		newEvent = metadata.EventGen()
		err = json.Unmarshal(data, newEvent)
		if err != nil {
			return fmt.Errorf("could not unmarshal %s into %T: %w", subscription.Type, newEvent, err)
		}
	}
	payloadContext := PayloadContext{
		Metadata:     message.Metadata,
		Subscription: message.Payload.Subscription,
	}

	switch event := newEvent.(type) {
	case *EventChannelUpdate:
		callFunc(c.onEventChannelUpdate, *event, payloadContext)
	case *EventChannelFollow:
		callFunc(c.onEventChannelFollow, *event, payloadContext)
	case *EventChannelSubscribe:
		callFunc(c.onEventChannelSubscribe, *event, payloadContext)
	case *EventChannelSubscriptionEnd:
		callFunc(c.onEventChannelSubscriptionEnd, *event, payloadContext)
	case *EventChannelSubscriptionGift:
		callFunc(c.onEventChannelSubscriptionGift, *event, payloadContext)
	case *EventChannelSubscriptionMessage:
		callFunc(c.onEventChannelSubscriptionMessage, *event, payloadContext)
	case *EventChannelCheer:
		callFunc(c.onEventChannelCheer, *event, payloadContext)
	case *EventChannelRaid:
		callFunc(c.onEventChannelRaid, *event, payloadContext)
	case *EventChannelBan:
		callFunc(c.onEventChannelBan, *event, payloadContext)
	case *EventChannelUnban:
		callFunc(c.onEventChannelUnban, *event, payloadContext)
	case *EventChannelModeratorAdd:
		callFunc(c.onEventChannelModeratorAdd, *event, payloadContext)
	case *EventChannelModeratorRemove:
		callFunc(c.onEventChannelModeratorRemove, *event, payloadContext)
	case *EventChannelVIPAdd:
		callFunc(c.onEventChannelVIPAdd, *event, payloadContext)
	case *EventChannelVIPRemove:
		callFunc(c.onEventChannelVIPRemove, *event, payloadContext)
	case *EventChannelChannelPointsCustomRewardAdd:
		callFunc(c.onEventChannelChannelPointsCustomRewardAdd, *event, payloadContext)
	case *EventChannelChannelPointsCustomRewardUpdate:
		callFunc(c.onEventChannelChannelPointsCustomRewardUpdate, *event, payloadContext)
	case *EventChannelChannelPointsCustomRewardRemove:
		callFunc(c.onEventChannelChannelPointsCustomRewardRemove, *event, payloadContext)
	case *EventChannelChannelPointsCustomRewardRedemptionAdd:
		callFunc(c.onEventChannelChannelPointsCustomRewardRedemptionAdd, *event, payloadContext)
	case *EventChannelChannelPointsCustomRewardRedemptionUpdate:
		callFunc(c.onEventChannelChannelPointsCustomRewardRedemptionUpdate, *event, payloadContext)
	case *EventChannelChannelPointsAutomaticRewardRedemptionAdd:
		callFunc(c.onEventChannelChannelPointsAutomaticRewardRedemptionAdd, *event, payloadContext)
	case *EventChannelPollBegin:
		callFunc(c.onEventChannelPollBegin, *event, payloadContext)
	case *EventChannelPollProgress:
		callFunc(c.onEventChannelPollProgress, *event, payloadContext)
	case *EventChannelPollEnd:
		callFunc(c.onEventChannelPollEnd, *event, payloadContext)
	case *EventChannelPredictionBegin:
		callFunc(c.onEventChannelPredictionBegin, *event, payloadContext)
	case *EventChannelPredictionProgress:
		callFunc(c.onEventChannelPredictionProgress, *event, payloadContext)
	case *EventChannelPredictionLock:
		callFunc(c.onEventChannelPredictionLock, *event, payloadContext)
	case *EventChannelPredictionEnd:
		callFunc(c.onEventChannelPredictionEnd, *event, payloadContext)
	case *[]EventDropEntitlementGrant:
		callFunc(c.onEventDropEntitlementGrant, *event, payloadContext)
	case *EventExtensionBitsTransactionCreate:
		callFunc(c.onEventExtensionBitsTransactionCreate, *event, payloadContext)
	case *EventChannelGoalBegin:
		callFunc(c.onEventChannelGoalBegin, *event, payloadContext)
	case *EventChannelGoalProgress:
		callFunc(c.onEventChannelGoalProgress, *event, payloadContext)
	case *EventChannelGoalEnd:
		callFunc(c.onEventChannelGoalEnd, *event, payloadContext)
	case *EventChannelHypeTrainBegin:
		callFunc(c.onEventChannelHypeTrainBegin, *event, payloadContext)
	case *EventChannelHypeTrainProgress:
		callFunc(c.onEventChannelHypeTrainProgress, *event, payloadContext)
	case *EventChannelHypeTrainEnd:
		callFunc(c.onEventChannelHypeTrainEnd, *event, payloadContext)
	case *EventStreamOnline:
		callFunc(c.onEventStreamOnline, *event, payloadContext)
	case *EventStreamOffline:
		callFunc(c.onEventStreamOffline, *event, payloadContext)
	case *EventUserAuthorizationGrant:
		callFunc(c.onEventUserAuthorizationGrant, *event, payloadContext)
	case *EventUserAuthorizationRevoke:
		callFunc(c.onEventUserAuthorizationRevoke, *event, payloadContext)
	case *EventUserUpdate:
		callFunc(c.onEventUserUpdate, *event, payloadContext)
	case *EventChannelCharityCampaignDonate:
		callFunc(c.onEventChannelCharityCampaignDonate, *event, payloadContext)
	case *EventChannelCharityCampaignProgress:
		callFunc(c.onEventChannelCharityCampaignProgress, *event, payloadContext)
	case *EventChannelCharityCampaignStart:
		callFunc(c.onEventChannelCharityCampaignStart, *event, payloadContext)
	case *EventChannelCharityCampaignStop:
		callFunc(c.onEventChannelCharityCampaignStop, *event, payloadContext)
	case *EventChannelShieldModeBegin:
		callFunc(c.onEventChannelShieldModeBegin, *event, payloadContext)
	case *EventChannelShieldModeEnd:
		callFunc(c.onEventChannelShieldModeEnd, *event, payloadContext)
	case *EventChannelShoutoutCreate:
		callFunc(c.onEventChannelShoutoutCreate, *event, payloadContext)
	case *EventChannelShoutoutReceive:
		callFunc(c.onEventChannelShoutoutReceive, *event, payloadContext)
	case *EventChannelModerate:
		callFunc(c.onEventChannelModerate, *event, payloadContext)
	case *EventChannelAdBreakBegin:
		callFunc(c.onEventChannelAdBreakBegin, *event, payloadContext)
	case *EventChannelWarningAcknowledge:
		callFunc(c.onEventChannelWarningAcknowledge, *event, payloadContext)
	case *EventChannelWarningSend:
		callFunc(c.onEventChannelWarningSend, *event, payloadContext)
	case *EventChannelUnbanRequestCreate:
		callFunc(c.onEventChannelUnbanRequestCreate, *event, payloadContext)
	case *EventChannelUnbanRequestResolve:
		callFunc(c.onEventChannelUnbanRequestResolve, *event, payloadContext)
	case *EventAutomodMessageHold:
		callFunc(c.onEventAutomodMessageHold, *event, payloadContext)
	case *EventAutomodMessageUpdate:
		callFunc(c.onEventAutomodMessageUpdate, *event, payloadContext)
	case *EventAutomodSettingsUpdate:
		callFunc(c.onEventAutomodSettingsUpdate, *event, payloadContext)
	case *EventAutomodTermsUpdate:
		callFunc(c.onEventAutomodTermsUpdate, *event, payloadContext)
	case *EventChannelChatUserMessageHold:
		callFunc(c.onEventChannelChatUserMessageHold, *event, payloadContext)
	case *EventChannelChatUserMessageUpdate:
		callFunc(c.onEventChannelChatUserMessageUpdate, *event, payloadContext)
	case *EventChannelChatClear:
		callFunc(c.onEventChannelChatClear, *event, payloadContext)
	case *EventChannelChatClearUserMessages:
		callFunc(c.onEventChannelChatClearUserMessages, *event, payloadContext)
	case *EventChannelChatMessage:
		callFunc(c.onEventChannelChatMessage, *event, payloadContext)
	case *EventChannelChatMessageDelete:
		callFunc(c.onEventChannelChatMessageDelete, *event, payloadContext)
	case *EventChannelChatNotification:
		callFunc(c.onEventChannelChatNotification, *event, payloadContext)
	case *EventChannelChatSettingsUpdate:
		callFunc(c.onEventChannelChatSettingsUpdate, *event, payloadContext)
	case *EventChannelSuspiciousUserMessage:
		callFunc(c.onEventChannelSuspiciousUserMessage, *event, payloadContext)
	case *EventChannelSuspiciousUserUpdate:
		callFunc(c.onEventChannelSuspiciousUserUpdate, *event, payloadContext)
	case *EventChannelSharedChatBegin:
		callFunc(c.onEventChannelSharedChatBegin, *event, payloadContext)
	case *EventChannelSharedChatUpdate:
		callFunc(c.onEventChannelSharedChatUpdate, *event, payloadContext)
	case *EventChannelSharedChatEnd:
		callFunc(c.onEventChannelSharedChatEnd, *event, payloadContext)
	case *EventUserWhisperMessage:
		callFunc(c.onEventUserWhisperMessage, *event, payloadContext)
	case *EventConduitShardDisabled:
		callFunc(c.onEventConduitShardDisabled, *event, payloadContext)
	default:
		c.onError(fmt.Errorf("unknown event type %s", subscription.Type))
	}

	return nil
}

func (c *Client) dial() (*websocket.Conn, error) {
	ws, _, err := websocket.Dial(c.ctx, c.Address, nil)
	if err != nil {
		return nil, fmt.Errorf("could not dial %s: %w", c.Address, err)
	}
	return ws, nil
}

func parseBaseMessage(data []byte) (MessageMetadata, error) {
	type BaseMessage struct {
		Metadata MessageMetadata `json:"metadata"`
	}

	var baseMessage BaseMessage
	err := json.Unmarshal(data, &baseMessage)
	if err != nil {
		return MessageMetadata{}, fmt.Errorf("could not unmarshal basemessage to get message type: %w", err)
	}

	return baseMessage.Metadata, nil
}

func (c *Client) OnError(callback func(err error)) {
	c.onError = callback
}

func (c *Client) OnWelcome(callback func(message WelcomeMessage, metadata MessageMetadata)) {
	c.onWelcome = callback
}

func (c *Client) OnKeepAlive(callback func(message KeepAliveMessage, metadata MessageMetadata)) {
	c.onKeepAlive = callback
}

func (c *Client) OnNotification(callback func(message NotificationMessage, metadata MessageMetadata)) {
	c.onNotification = callback
}

func (c *Client) OnReconnect(callback func(message ReconnectMessage, metadata MessageMetadata)) {
	c.onReconnect = callback
}

func (c *Client) OnRevoke(callback func(message RevokeMessage, metadata MessageMetadata)) {
	c.onRevoke = callback
}

func (c *Client) OnRawEvent(callback func(event string, metadata MessageMetadata, subscription PayloadSubscription)) {
	c.onRawEvent = callback
}

func (c *Client) OnEventChannelUpdate(callback func(event EventChannelUpdate, payloadContext PayloadContext)) {
	c.onEventChannelUpdate = callback
}

func (c *Client) OnEventChannelFollow(callback func(event EventChannelFollow, payloadContext PayloadContext)) {
	c.onEventChannelFollow = callback
}

func (c *Client) OnEventChannelSubscribe(callback func(event EventChannelSubscribe, payloadContext PayloadContext)) {
	c.onEventChannelSubscribe = callback
}

func (c *Client) OnEventChannelSubscriptionEnd(callback func(event EventChannelSubscriptionEnd, payloadContext PayloadContext)) {
	c.onEventChannelSubscriptionEnd = callback
}

func (c *Client) OnEventChannelSubscriptionGift(callback func(event EventChannelSubscriptionGift, payloadContext PayloadContext)) {
	c.onEventChannelSubscriptionGift = callback
}

func (c *Client) OnEventChannelSubscriptionMessage(callback func(event EventChannelSubscriptionMessage, payloadContext PayloadContext)) {
	c.onEventChannelSubscriptionMessage = callback
}

func (c *Client) OnEventChannelCheer(callback func(event EventChannelCheer, payloadContext PayloadContext)) {
	c.onEventChannelCheer = callback
}

func (c *Client) OnEventChannelRaid(callback func(event EventChannelRaid, payloadContext PayloadContext)) {
	c.onEventChannelRaid = callback
}

func (c *Client) OnEventChannelBan(callback func(event EventChannelBan, payloadContext PayloadContext)) {
	c.onEventChannelBan = callback
}

func (c *Client) OnEventChannelUnban(callback func(event EventChannelUnban, payloadContext PayloadContext)) {
	c.onEventChannelUnban = callback
}

func (c *Client) OnEventChannelModeratorAdd(callback func(event EventChannelModeratorAdd, payloadContext PayloadContext)) {
	c.onEventChannelModeratorAdd = callback
}

func (c *Client) OnEventChannelModeratorRemove(callback func(event EventChannelModeratorRemove, payloadContext PayloadContext)) {
	c.onEventChannelModeratorRemove = callback
}

func (c *Client) OnEventChannelVIPAdd(callback func(event EventChannelVIPAdd, payloadContext PayloadContext)) {
	c.onEventChannelVIPAdd = callback
}

func (c *Client) OnEventChannelVIPRemove(callback func(event EventChannelVIPRemove, payloadContext PayloadContext)) {
	c.onEventChannelVIPRemove = callback
}

func (c *Client) OnEventChannelChannelPointsCustomRewardAdd(callback func(event EventChannelChannelPointsCustomRewardAdd, payloadContext PayloadContext)) {
	c.onEventChannelChannelPointsCustomRewardAdd = callback
}

func (c *Client) OnEventChannelChannelPointsCustomRewardUpdate(callback func(event EventChannelChannelPointsCustomRewardUpdate, payloadContext PayloadContext)) {
	c.onEventChannelChannelPointsCustomRewardUpdate = callback
}

func (c *Client) OnEventChannelChannelPointsCustomRewardRemove(callback func(event EventChannelChannelPointsCustomRewardRemove, payloadContext PayloadContext)) {
	c.onEventChannelChannelPointsCustomRewardRemove = callback
}

func (c *Client) OnEventChannelChannelPointsCustomRewardRedemptionAdd(callback func(event EventChannelChannelPointsCustomRewardRedemptionAdd, payloadContext PayloadContext)) {
	c.onEventChannelChannelPointsCustomRewardRedemptionAdd = callback
}

func (c *Client) OnEventChannelChannelPointsCustomRewardRedemptionUpdate(callback func(event EventChannelChannelPointsCustomRewardRedemptionUpdate, payloadContext PayloadContext)) {
	c.onEventChannelChannelPointsCustomRewardRedemptionUpdate = callback
}

func (c *Client) OnEventChannelChannelPointsAutomaticRewardRedemptionAdd(callback func(event EventChannelChannelPointsAutomaticRewardRedemptionAdd, payloadContext PayloadContext)) {
	c.onEventChannelChannelPointsAutomaticRewardRedemptionAdd = callback
}

func (c *Client) OnEventChannelPollBegin(callback func(event EventChannelPollBegin, payloadContext PayloadContext)) {
	c.onEventChannelPollBegin = callback
}

func (c *Client) OnEventChannelPollProgress(callback func(event EventChannelPollProgress, payloadContext PayloadContext)) {
	c.onEventChannelPollProgress = callback
}

func (c *Client) OnEventChannelPollEnd(callback func(event EventChannelPollEnd, payloadContext PayloadContext)) {
	c.onEventChannelPollEnd = callback
}

func (c *Client) OnEventChannelPredictionBegin(callback func(event EventChannelPredictionBegin, payloadContext PayloadContext)) {
	c.onEventChannelPredictionBegin = callback
}

func (c *Client) OnEventChannelPredictionProgress(callback func(event EventChannelPredictionProgress, payloadContext PayloadContext)) {
	c.onEventChannelPredictionProgress = callback
}

func (c *Client) OnEventChannelPredictionLock(callback func(event EventChannelPredictionLock, payloadContext PayloadContext)) {
	c.onEventChannelPredictionLock = callback
}

func (c *Client) OnEventChannelPredictionEnd(callback func(event EventChannelPredictionEnd, payloadContext PayloadContext)) {
	c.onEventChannelPredictionEnd = callback
}

func (c *Client) OnEventDropEntitlementGrant(callback func(event []EventDropEntitlementGrant, payloadContext PayloadContext)) {
	c.onEventDropEntitlementGrant = callback
}

func (c *Client) OnEventExtensionBitsTransactionCreate(callback func(event EventExtensionBitsTransactionCreate, payloadContext PayloadContext)) {
	c.onEventExtensionBitsTransactionCreate = callback
}

func (c *Client) OnEventChannelGoalBegin(callback func(event EventChannelGoalBegin, payloadContext PayloadContext)) {
	c.onEventChannelGoalBegin = callback
}

func (c *Client) OnEventChannelGoalProgress(callback func(event EventChannelGoalProgress, payloadContext PayloadContext)) {
	c.onEventChannelGoalProgress = callback
}

func (c *Client) OnEventChannelGoalEnd(callback func(event EventChannelGoalEnd, payloadContext PayloadContext)) {
	c.onEventChannelGoalEnd = callback
}

func (c *Client) OnEventChannelHypeTrainBegin(callback func(event EventChannelHypeTrainBegin, payloadContext PayloadContext)) {
	c.onEventChannelHypeTrainBegin = callback
}

func (c *Client) OnEventChannelHypeTrainProgress(callback func(event EventChannelHypeTrainProgress, payloadContext PayloadContext)) {
	c.onEventChannelHypeTrainProgress = callback
}

func (c *Client) OnEventChannelHypeTrainEnd(callback func(event EventChannelHypeTrainEnd, payloadContext PayloadContext)) {
	c.onEventChannelHypeTrainEnd = callback
}

func (c *Client) OnEventStreamOnline(callback func(event EventStreamOnline, payloadContext PayloadContext)) {
	c.onEventStreamOnline = callback
}

func (c *Client) OnEventStreamOffline(callback func(event EventStreamOffline, payloadContext PayloadContext)) {
	c.onEventStreamOffline = callback
}

func (c *Client) OnEventUserAuthorizationGrant(callback func(event EventUserAuthorizationGrant, payloadContext PayloadContext)) {
	c.onEventUserAuthorizationGrant = callback
}

func (c *Client) OnEventUserAuthorizationRevoke(callback func(event EventUserAuthorizationRevoke, payloadContext PayloadContext)) {
	c.onEventUserAuthorizationRevoke = callback
}

func (c *Client) OnEventUserUpdate(callback func(event EventUserUpdate, payloadContext PayloadContext)) {
	c.onEventUserUpdate = callback
}

func (c *Client) OnEventChannelCharityCampaignDonate(callback func(event EventChannelCharityCampaignDonate, payloadContext PayloadContext)) {
	c.onEventChannelCharityCampaignDonate = callback
}

func (c *Client) OnEventChannelCharityCampaignProgress(callback func(event EventChannelCharityCampaignProgress, payloadContext PayloadContext)) {
	c.onEventChannelCharityCampaignProgress = callback
}

func (c *Client) OnEventChannelCharityCampaignStart(callback func(event EventChannelCharityCampaignStart, payloadContext PayloadContext)) {
	c.onEventChannelCharityCampaignStart = callback
}

func (c *Client) OnEventChannelCharityCampaignStop(callback func(event EventChannelCharityCampaignStop, payloadContext PayloadContext)) {
	c.onEventChannelCharityCampaignStop = callback
}

func (c *Client) OnEventChannelShieldModeBegin(callback func(event EventChannelShieldModeBegin, payloadContext PayloadContext)) {
	c.onEventChannelShieldModeBegin = callback
}

func (c *Client) OnEventChannelShieldModeEnd(callback func(event EventChannelShieldModeEnd, payloadContext PayloadContext)) {
	c.onEventChannelShieldModeEnd = callback
}

func (c *Client) OnEventChannelShoutoutCreate(callback func(event EventChannelShoutoutCreate, payloadContext PayloadContext)) {
	c.onEventChannelShoutoutCreate = callback
}

func (c *Client) OnEventChannelShoutoutReceive(callback func(event EventChannelShoutoutReceive, payloadContext PayloadContext)) {
	c.onEventChannelShoutoutReceive = callback
}

func (c *Client) OnEventChannelModerate(callback func(event EventChannelModerate, payloadContext PayloadContext)) {
	c.onEventChannelModerate = callback
}

func (c *Client) OnEventChannelAdBreakBegin(callback func(event EventChannelAdBreakBegin, payloadContext PayloadContext)) {
	c.onEventChannelAdBreakBegin = callback
}

func (c *Client) OnEventChannelWarningAcknowledge(callback func(event EventChannelWarningAcknowledge, payloadContext PayloadContext)) {
	c.onEventChannelWarningAcknowledge = callback
}

func (c *Client) OnEventChannelWarningSend(callback func(event EventChannelWarningSend, payloadContext PayloadContext)) {
	c.onEventChannelWarningSend = callback
}

func (c *Client) OnEventChannelUnbanRequestCreate(callback func(event EventChannelUnbanRequestCreate, payloadContext PayloadContext)) {
	c.onEventChannelUnbanRequestCreate = callback
}

func (c *Client) OnEventChannelUnbanRequestResolve(callback func(event EventChannelUnbanRequestResolve, payloadContext PayloadContext)) {
	c.onEventChannelUnbanRequestResolve = callback
}

func (c *Client) OnEventAutomodMessageHold(callback func(event EventAutomodMessageHold, payloadContext PayloadContext)) {
	c.onEventAutomodMessageHold = callback
}

func (c *Client) OnEventAutomodMessageUpdate(callback func(event EventAutomodMessageUpdate, payloadContext PayloadContext)) {
	c.onEventAutomodMessageUpdate = callback
}

func (c *Client) OnEventAutomodSettingsUpdate(callback func(event EventAutomodSettingsUpdate, payloadContext PayloadContext)) {
	c.onEventAutomodSettingsUpdate = callback
}

func (c *Client) OnEventAutomodTermsUpdate(callback func(event EventAutomodTermsUpdate, payloadContext PayloadContext)) {
	c.onEventAutomodTermsUpdate = callback
}

func (c *Client) OnEventChannelChatUserMessageHold(callback func(event EventChannelChatUserMessageHold, payloadContext PayloadContext)) {
	c.onEventChannelChatUserMessageHold = callback
}

func (c *Client) OnEventChannelChatUserMessageUpdate(callback func(event EventChannelChatUserMessageUpdate, payloadContext PayloadContext)) {
	c.onEventChannelChatUserMessageUpdate = callback
}

func (c *Client) OnEventChannelChatClear(callback func(event EventChannelChatClear, payloadContext PayloadContext)) {
	c.onEventChannelChatClear = callback
}

func (c *Client) OnEventChannelChatClearUserMessages(callback func(event EventChannelChatClearUserMessages, payloadContext PayloadContext)) {
	c.onEventChannelChatClearUserMessages = callback
}

func (c *Client) OnEventChannelChatMessage(callback func(event EventChannelChatMessage, payloadContext PayloadContext)) {
	c.onEventChannelChatMessage = callback
}

func (c *Client) OnEventChannelChatMessageDelete(callback func(event EventChannelChatMessageDelete, payloadContext PayloadContext)) {
	c.onEventChannelChatMessageDelete = callback
}

func (c *Client) OnEventChannelChatNotification(callback func(event EventChannelChatNotification, payloadContext PayloadContext)) {
	c.onEventChannelChatNotification = callback
}

func (c *Client) OnEventChannelChatSettingsUpdate(callback func(event EventChannelChatSettingsUpdate, payloadContext PayloadContext)) {
	c.onEventChannelChatSettingsUpdate = callback
}

func (c *Client) OnEventChannelSuspiciousUserMessage(callback func(event EventChannelSuspiciousUserMessage, payloadContext PayloadContext)) {
	c.onEventChannelSuspiciousUserMessage = callback
}

func (c *Client) OnEventChannelSuspiciousUserUpdate(callback func(event EventChannelSuspiciousUserUpdate, payloadContext PayloadContext)) {
	c.onEventChannelSuspiciousUserUpdate = callback
}

func (c *Client) OnEventChannelSharedChatBegin(callback func(event EventChannelSharedChatBegin, payloadContext PayloadContext)) {
	c.onEventChannelSharedChatBegin = callback
}

func (c *Client) OnEventChannelSharedChatUpdate(callback func(event EventChannelSharedChatUpdate, payloadContext PayloadContext)) {
	c.onEventChannelSharedChatUpdate = callback
}

func (c *Client) OnEventChannelSharedChatEnd(callback func(event EventChannelSharedChatEnd, payloadContext PayloadContext)) {
	c.onEventChannelSharedChatEnd = callback
}

func (c *Client) OnEventUserWhisperMessage(callback func(event EventUserWhisperMessage, payloadContext PayloadContext)) {
	c.onEventUserWhisperMessage = callback
}

func (c *Client) OnEventConduitShardDisabled(callback func(event EventConduitShardDisabled, payloadContext PayloadContext)) {
	c.onEventConduitShardDisabled = callback
}
