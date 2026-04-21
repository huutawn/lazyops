package service

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"log/slog"
	"net"
	neturl "net/url"
	"strings"
	"time"
)

const (
	publicURLStatusReady   = "ready"
	publicURLStatusPending = "pending"
	publicURLStatusError   = "error"
)

type PublicURLTLSObservation struct {
	URL       string
	Host      string
	Status    string
	Reason    string
	ErrorKind string
}

type PublicURLVerifier interface {
	Observe(context.Context, string) PublicURLTLSObservation
}

type SystemPublicURLVerifier struct {
	dnsTimeout  time.Duration
	dialTimeout time.Duration
}

func NewSystemPublicURLVerifier() *SystemPublicURLVerifier {
	return &SystemPublicURLVerifier{
		dnsTimeout:  1500 * time.Millisecond,
		dialTimeout: 2 * time.Second,
	}
}

func (v *SystemPublicURLVerifier) Observe(ctx context.Context, rawURL string) PublicURLTLSObservation {
	observation := PublicURLTLSObservation{
		URL:    strings.TrimSpace(rawURL),
		Status: publicURLStatusPending,
	}
	if observation.URL == "" {
		observation.Reason = "Chưa có public URL để kiểm tra."
		observation.ErrorKind = "missing_url"
		return observation
	}

	parsed, err := neturl.Parse(observation.URL)
	if err != nil {
		observation.Status = publicURLStatusError
		observation.Reason = "Public URL không hợp lệ nên không thể kiểm tra TLS."
		observation.ErrorKind = "invalid_url"
		return observation
	}
	observation.Host = strings.TrimSpace(parsed.Hostname())
	if observation.Host == "" {
		observation.Status = publicURLStatusError
		observation.Reason = "Public URL không có hostname hợp lệ."
		observation.ErrorKind = "missing_host"
		return observation
	}

	resolveCtx, cancelResolve := context.WithTimeout(ctx, v.dnsTimeout)
	defer cancelResolve()
	ips, err := net.DefaultResolver.LookupIPAddr(resolveCtx, observation.Host)
	if err != nil || len(ips) == 0 {
		observation.Status = publicURLStatusPending
		observation.Reason = "Đang chờ DNS cho magic domain hoạt động."
		observation.ErrorKind = "dns_unresolved"
		return observation
	}

	port := parsed.Port()
	if port == "" {
		port = "443"
	}

	dialCtx, cancelDial := context.WithTimeout(ctx, v.dialTimeout)
	defer cancelDial()
	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: v.dialTimeout},
		Config: &tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: observation.Host,
		},
	}
	conn, err := dialer.DialContext(dialCtx, "tcp", net.JoinHostPort(observation.Host, port))
	if err != nil {
		observation.Status, observation.Reason, observation.ErrorKind = classifyPublicURLTLSError(err)
		return observation
	}
	defer conn.Close()

	tlsConn, ok := conn.(*tls.Conn)
	if ok {
		state := tlsConn.ConnectionState()
		if len(state.PeerCertificates) == 0 {
			observation.Status = publicURLStatusError
			observation.Reason = "Gateway HTTPS không trình ra certificate hợp lệ."
			observation.ErrorKind = "missing_certificate"
			return observation
		}
	}

	observation.Status = publicURLStatusReady
	observation.Reason = ""
	observation.ErrorKind = ""
	return observation
}

func classifyPublicURLTLSError(err error) (status, reason, kind string) {
	lower := strings.ToLower(strings.TrimSpace(err.Error()))
	var unknownAuthority x509.UnknownAuthorityError
	var hostnameErr x509.HostnameError

	switch {
	case errors.As(err, &unknownAuthority) || strings.Contains(lower, "unknown authority"):
		return publicURLStatusError, "Magic domain đang trả chứng chỉ local CA, trình duyệt sẽ báo không an toàn.", "unknown_authority"
	case errors.As(err, &hostnameErr) || strings.Contains(lower, "certificate is valid for") || strings.Contains(lower, "not valid for"):
		return publicURLStatusError, "Chứng chỉ TLS của magic domain không khớp hostname.", "hostname_mismatch"
	case errors.Is(err, context.DeadlineExceeded),
		strings.Contains(lower, "deadline exceeded"),
		strings.Contains(lower, "i/o timeout"),
		strings.Contains(lower, "connection refused"),
		strings.Contains(lower, "no such host"),
		strings.Contains(lower, "network is unreachable"),
		strings.Contains(lower, "server misbehaving"):
		return publicURLStatusPending, "Magic domain chưa reachable trên cổng 443.", "network_pending"
	default:
		return publicURLStatusPending, "Đang chờ cấp chứng chỉ TLS công khai cho magic domain.", "tls_pending"
	}
}

func summarizePublicURLObservations(observations []PublicURLTLSObservation) (string, string) {
	if len(observations) == 0 {
		return "", ""
	}
	firstPending := ""
	firstError := ""
	for _, item := range observations {
		switch strings.TrimSpace(item.Status) {
		case publicURLStatusReady:
			return publicURLStatusReady, ""
		case publicURLStatusError:
			if firstError == "" {
				firstError = strings.TrimSpace(item.Reason)
			}
		case publicURLStatusPending:
			if firstPending == "" {
				firstPending = strings.TrimSpace(item.Reason)
			}
		}
	}
	if firstError != "" {
		return publicURLStatusError, firstError
	}
	if firstPending != "" {
		return publicURLStatusPending, firstPending
	}
	return "", ""
}

func logFailedPublicURLObservation(observation PublicURLTLSObservation) {
	if strings.TrimSpace(observation.Status) == publicURLStatusReady {
		return
	}
	slog.Default().Warn(
		"public_url_tls_check_failed",
		"url", observation.URL,
		"host", observation.Host,
		"status", observation.Status,
		"error_kind", observation.ErrorKind,
		"reason", observation.Reason,
	)
}
