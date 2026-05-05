package mqtt

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	authorization "smart-pc-waker-agent/internal/auth"
	"smart-pc-waker-agent/internal/config"
	"smart-pc-waker-agent/internal/lib/random"
	powerOn "smart-pc-waker-agent/internal/mqtt/commands/power-on"
	configStorage "smart-pc-waker-agent/internal/storage/config-storage"
	"strings"

	"github.com/MaxRomanov007/smart-pc-go-lib/commands"
	commandMessage "github.com/MaxRomanov007/smart-pc-go-lib/domain/models/command-message"
	mqttAuth "github.com/MaxRomanov007/smart-pc-go-lib/mqtt-auth"
)

const clientIDPostfixLength = 6

type MQTT struct {
	Connection *mqttAuth.Connection
}

type PcIDGetter interface {
	GetPcID(ctx context.Context) (string, error)
}

func New(
	ctx context.Context,
	log *slog.Logger,
	mqttCfg config.MQTT,
	auth *authorization.Auth,
	storage *configStorage.Storage,
) (*MQTT, error) {
	const op = "mqtt.New"

	mqttConnCfg, router, err := createMQTTConfig(ctx, mqttCfg, auth)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to create mqtt config: %w", op, err)
	}

	connection, err := mqttAuth.NewConnection(ctx, mqttConnCfg)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to create mqtt connection: %w", op, err)
	}

	executor := commands.NewExecutor(connection, router)
	executor.Set("power-on", powerOn.New(log, storage))

	if err := executor.StartListen(ctx, &commands.StartListenOptions{
		CommandTopic:       "pcs/+/command",
		CommandMessageType: "waker-command",
		LogTopicFunc: func(msg *commandMessage.Message) string {
			parts := strings.Split(msg.Publish.Topic, "/")
			if len(parts) < 2 {
				return ""
			}

			return fmt.Sprintf("pcs/%s/log", parts[len(parts)-2])
		},
		LogMessageType: "waker-command-log",
		Log:            log,
	}); err != nil {
		return nil, fmt.Errorf("%s: failed to start listening commands: %w", op, err)
	}

	return &MQTT{
		Connection: connection,
	}, nil
}

func createMQTTConfig(
	ctx context.Context,
	mqttCfg config.MQTT,
	auth *authorization.Auth,
) (*mqttAuth.ClientConfig, *mqttAuth.Router, error) {
	const op = "mqtt.createMQTTConfig"

	a, err := auth.Inner()
	if err != nil {
		return nil, nil, fmt.Errorf("%s: failed to get auth: %w", op, err)
	}

	cfg, router, err := mqttAuth.NewClientConfigWithRouter(ctx, a)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"%s: failed to create new client config with router: %w",
			op,
			err,
		)
	}

	broker, err := url.Parse(mqttCfg.BrokerURL)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: failed to parse broker url: %w", op, err)
	}

	cfg.ClientConfig.ClientID = mqttCfg.ClientIDPrefix + random.String(clientIDPostfixLength)
	cfg.ServerUrls = []*url.URL{broker}
	cfg.CleanStartOnInitialConnection = false
	cfg.SessionExpiryInterval = mqttCfg.SessionExpiryInterval
	cfg.KeepAlive = mqttCfg.KeepAlive

	return cfg, router, nil
}

func (m *MQTT) Done() <-chan struct{} {
	return m.Connection.Done()
}
