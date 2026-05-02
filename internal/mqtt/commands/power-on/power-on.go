package powerOn

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"smart-pc-waker-agent/internal/lib/wol"
	"smart-pc-waker-agent/internal/storage"
	"strings"

	"github.com/MaxRomanov007/smart-pc-go-lib/commands"
	commandMessage "github.com/MaxRomanov007/smart-pc-go-lib/domain/models/command-message"
	"github.com/MaxRomanov007/smart-pc-go-lib/logger/sl"
)

type MACGetter interface {
	GetMACByPcId(ctx context.Context, pcId string) (string, error)
}

func New(log *slog.Logger, macGetter MACGetter) commands.CommandFunc {
	return func(ctx context.Context, msg *commandMessage.Message) error {
		const op = "commands.power-on"

		log := log.With(sl.Op(op), sl.MsgID(msg.Publish))

		parts := strings.Split(msg.Publish.Topic, "/")
		if len(parts) < 2 {
			return fmt.Errorf("invalid topic %q", msg.Publish.Topic)
		}
		pcId := parts[len(parts)-2]

		macAddr, err := macGetter.GetMACByPcId(ctx, pcId)
		if errors.Is(err, storage.ErrNotFound) {
			log.Warn("MAC address not found")
			return commands.Error("MAC not found")
		}
		if err != nil {
			log.Warn("failed to get MAC by pcId", sl.Err(err))
			return fmt.Errorf("%s: failed to get MAC: %w", op, err)
		}

		if err := sendWoL(macAddr); err != nil {
			log.Warn("failed to send WoL packet", sl.Err(err))
			return fmt.Errorf("%s: failed to send WoL: %w", op, err)
		}

		log.Info("WoL magic packet sent", slog.String("pcId", pcId))
		return nil
	}
}

// sendWoL отправляет Magic Packet на все доступные broadcast-адреса.
// Пробует directed broadcast каждого интерфейса + глобальный 255.255.255.255,
// на портах 9 и 7. Возвращает ошибку только если ни один вариант не сработал.
func sendWoL(macAddr string) error {
	mp, err := wol.New(macAddr)
	if err != nil {
		return fmt.Errorf("invalid MAC: %w", err)
	}

	bs, err := mp.Marshal()
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}

	targets := broadcastTargets()

	var lastErr error
	sent := false

	for _, target := range targets {
		for _, port := range []string{"9", "7"} {
			addr := fmt.Sprintf("%s:%s", target, port)
			if err := sendPacket(bs, addr); err != nil {
				lastErr = err
				continue
			}
			sent = true
		}
	}

	if !sent {
		return fmt.Errorf("failed to send WoL packet to any target: %w", lastErr)
	}

	return nil
}

// sendPacket отправляет байты на указанный UDP-адрес.
func sendPacket(bs []byte, addr string) error {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}

	// Используем ListenPacket вместо DialUDP — это позволяет
	// корректно отправлять на broadcast-адреса на всех платформах.
	conn, err := net.ListenPacket("udp", ":0")
	if err != nil {
		return err
	}
	defer conn.Close()

	n, err := conn.WriteTo(bs, udpAddr)
	if err != nil {
		return err
	}
	if n != 102 {
		return fmt.Errorf("sent %d bytes, expected 102", n)
	}

	return nil
}

// broadcastTargets возвращает список broadcast-адресов:
// directed broadcast для каждого сетевого интерфейса + глобальный fallback.
func broadcastTargets() []string {
	var targets []string

	ifaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			// Пропускаем loopback и выключенные интерфейсы
			if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
				continue
			}
			if addr, err := getBroadcastAddr(iface); err == nil {
				targets = append(targets, addr)
			}
		}
	}

	// Глобальный broadcast как fallback
	targets = append(targets, "255.255.255.255")

	return targets
}

// getBroadcastAddr вычисляет directed broadcast адрес для интерфейса.
func getBroadcastAddr(iface net.Interface) (string, error) {
	addrs, err := iface.Addrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		ip := ipNet.IP.To4()
		if ip == nil {
			continue // пропускаем IPv6
		}

		broadcast := make(net.IP, 4)
		for i := range ip {
			broadcast[i] = ip[i] | ^ipNet.Mask[i]
		}
		return broadcast.String(), nil
	}

	return "", fmt.Errorf("no IPv4 address on interface %s", iface.Name)
}
