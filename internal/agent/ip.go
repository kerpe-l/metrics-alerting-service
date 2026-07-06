package agent

import (
	"fmt"
	"net"
)

// OutboundIP определяет исходящий IP-адрес агента для связи с сервером по addr
// (host:port). Реального соединения нет: UDP-«dial» лишь заставляет ядро выбрать
// маршрут и локальный адрес источника, который и возвращается.
func OutboundIP(addr string) (string, error) {
	conn, err := net.Dial("udp", addr)
	if err != nil {
		return "", fmt.Errorf("определение исходящего IP: %w", err)
	}
	defer func() { _ = conn.Close() }()

	return conn.LocalAddr().(*net.UDPAddr).IP.String(), nil
}
