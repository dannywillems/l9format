package l9format

import (
	"context"
	"crypto/tls"
	"fmt"
	"gitlab.nobody.run/tbi/socksme"
	"net"
	"net/http"
	"strings"
	"time"
)

type ServicePluginInterface interface {
	GetVersion() (int, int, int)
	GetProtocols() []string
	GetName() string
	GetStage() string
	Run(ctx context.Context, event *L9Event) (leak L9LeakEvent, hasLeak bool)
}

type ServicePluginBase struct {
}

func (plugin ServicePluginBase) GetL9NetworkConnection(event *L9Event) (conn net.Conn, err error) {
	network := "tcp"
	if event.HasTransport("udp") {
		network = "udp"
	}
	addr := net.JoinHostPort(event.Ip, event.Port)
	return plugin.DialContext(nil, network, addr)
}

func (plugin ServicePluginBase) GetNetworkConnection(network string, addr string) (conn net.Conn, err error) {
	return plugin.DialContext(nil, network, addr)
}

func (plugin ServicePluginBase) DialContext(ctx context.Context, network string, addr string) (conn net.Conn, err error) {
	if ctx != nil {
		deadline, hasDeadline := ctx.Deadline()
		if hasDeadline {
			conn, err = net.DialTimeout(network, addr, deadline.Sub(time.Now()))
		} else {
			conn, err = net.DialTimeout(network, addr, 3*time.Second)
		}
	} else {
		conn, err = net.DialTimeout(network, addr, 3*time.Second)
	}
	if tcpConn, isTcp := conn.(*net.TCPConn); isTcp {
		// Will considerably lower TIME_WAIT connections and required fds,
		// since we don't plan to reconnect to the same host:port combo and need TIME_WAIT's window anyway
		// Will lead to out of sequence events if used on the same target host/port and source port starts to collide.
		// TLDR : DO NOT USE ON AN HOST THAT'S NOT DEDICATED TO SCANNING
		_ = tcpConn.SetLinger(0)

	}
	return conn, err
	// If you want to use a socks proxy ... Making network.go its own library soon.
	//TODO : implement socks proxy support
	return socksme.NewDialer("tcp", fmt.Sprintf("127.0.0.1:2250")).
		DialContext(ctx, "tcp", addr)
}

func (plugin ServicePluginBase) GetHttpClient(ctx context.Context, ip string, port string) *http.Client {
	if strings.Contains(ip, ":") && !strings.Contains(ip, "[") {
		ip = fmt.Sprintf("[%s]", ip)
	}
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _ string, _ string) (net.Conn, error) {
				addr := ip + ":" + port
				return plugin.DialContext(ctx, "tcp", addr)
			},
			ResponseHeaderTimeout: 2 * time.Second,
			ExpectContinueTimeout: 2 * time.Second,
			DisableKeepAlives:     true,
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
