package main

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

const defaultAddress = "127.0.0.1:19081"

func resolveAddress(explicit, portEnvironment string) (string, error) {
	address := strings.TrimSpace(explicit)
	if address == "" && strings.TrimSpace(portEnvironment) != "" {
		port, err := strconv.Atoi(strings.TrimSpace(portEnvironment))
		if err != nil || port < 1 || port > 65535 {
			return "", errors.New("PORT 必须是 1 到 65535 的端口号")
		}
		address = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	}
	if address == "" {
		address = defaultAddress
	}

	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return "", fmt.Errorf("-addr 格式无效: %w", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return "", errors.New("-addr 端口必须在 1 到 65535 之间")
	}
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return "", errors.New("-addr 必须绑定回环地址")
	}
	return address, nil
}
