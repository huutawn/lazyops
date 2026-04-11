// Package runtime provides an embedded DNS server for LazyOps service discovery.
// It resolves service hostnames like *.service.lazyops.internal to their
// actual endpoints, enabling convention-based service communication without
// requiring env var injection or sidecar proxy interception.
//
// Service Resolution Patterns:
//   - <service>.<project>.lazyops.internal  → service IP:port
//   - <service>.lazyops.internal            → service IP:port (short form)
//   - _http._tcp.<service>.lazyops.internal → SRV record for HTTP services
package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultDomain is the base domain for LazyOps internal DNS
	DefaultDomain = "lazyops.internal"

	// ServiceDomain is the subdomain for service discovery
	ServiceDomain = "service." + DefaultDomain

	// DefaultListenAddr is the default DNS server listen address
	DefaultListenAddr = "127.0.0.1:5353"
)

// RecordType represents a DNS record type
type RecordType uint16

const (
	RecordTypeA    RecordType = 1
	RecordTypeAAAA RecordType = 28
	RecordTypeSRV  RecordType = 33
)

// DNSRecord represents a single DNS record entry
type DNSRecord struct {
	Name       string
	RecordType RecordType
	Value      string
	TTL        time.Duration
	CreatedAt  time.Time
}

// ServiceRecord holds service discovery information
type ServiceRecord struct {
	ServiceName string
	ProjectID   string
	Host        string
	Port        int
	Protocol    string // "http", "tcp", "grpc"
	HealthCheck string
}

// DNSServer is an embedded DNS server for LazyOps service discovery
type DNSServer struct {
	logger     *slog.Logger
	listenAddr string
	listener   *net.UDPConn
	mu         sync.RWMutex
	records    map[string]*DNSRecord
	services   map[string]*ServiceRecord
	domain     string
	started    bool
	done       chan struct{}
}

// NewDNSServer creates a new embedded DNS server
func NewDNSServer(logger *slog.Logger, listenAddr string) *DNSServer {
	if listenAddr == "" {
		listenAddr = DefaultListenAddr
	}
	return &DNSServer{
		logger:     logger,
		listenAddr: listenAddr,
		records:    make(map[string]*DNSRecord),
		services:   make(map[string]*ServiceRecord),
		domain:     DefaultDomain,
		done:       make(chan struct{}),
	}
}

// RegisterService registers a service with the DNS server
func (d *DNSServer) RegisterService(service ServiceRecord) {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := serviceKey(service.ServiceName, service.ProjectID)
	d.services[key] = &service

	// Register full hostname
	hostname := fmt.Sprintf("%s.%s.%s", service.ServiceName, service.ProjectID, d.domain)
	d.records[hostname] = &DNSRecord{
		Name:       hostname,
		RecordType: RecordTypeA,
		Value:      service.Host,
		TTL:        30 * time.Second,
		CreatedAt:  time.Now(),
	}

	// Register short form (service.lazyops.internal)
	shortHostname := fmt.Sprintf("%s.%s", service.ServiceName, d.domain)
	d.records[shortHostname] = &DNSRecord{
		Name:       shortHostname,
		RecordType: RecordTypeA,
		Value:      service.Host,
		TTL:        30 * time.Second,
		CreatedAt:  time.Now(),
	}

	if d.logger != nil {
		d.logger.Info("registered DNS record for service",
			"service", service.ServiceName,
			"project", service.ProjectID,
			"hostname", hostname,
			"ip", service.Host,
			"port", service.Port,
		)
	}
}

// UnregisterService removes a service from the DNS server
func (d *DNSServer) UnregisterService(serviceName, projectID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := serviceKey(serviceName, projectID)
	delete(d.services, key)

	hostname := fmt.Sprintf("%s.%s.%s", serviceName, projectID, d.domain)
	delete(d.records, hostname)

	shortHostname := fmt.Sprintf("%s.%s", serviceName, d.domain)
	delete(d.records, shortHostname)

	if d.logger != nil {
		d.logger.Info("unregistered DNS record for service",
			"service", serviceName,
			"project", projectID,
		)
	}
}

// Lookup resolves a hostname to an IP address
func (d *DNSServer) Lookup(name string) (string, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Normalize name (remove trailing dot)
	name = strings.TrimSuffix(name, ".")
	name = strings.ToLower(name)

	record, ok := d.records[name]
	if !ok {
		return "", false
	}
	return record.Value, true
}

// ListServices returns all registered services
func (d *DNSServer) ListServices() []ServiceRecord {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]ServiceRecord, 0, len(d.services))
	for _, svc := range d.services {
		result = append(result, *svc)
	}
	return result
}

// Start starts the DNS server
func (d *DNSServer) Start(ctx context.Context) error {
	d.mu.Lock()
	if d.started {
		d.mu.Unlock()
		return fmt.Errorf("DNS server already started")
	}
	d.started = true
	d.mu.Unlock()

	addr, err := net.ResolveUDPAddr("udp", d.listenAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve DNS address: %w", err)
	}

	d.listener, err = net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to start DNS server: %w", err)
	}

	go d.serve(ctx)

	if d.logger != nil {
		d.logger.Info("DNS server started",
			"addr", d.listenAddr,
			"domain", d.domain,
		)
	}

	return nil
}

// Stop stops the DNS server
func (d *DNSServer) Stop() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.started {
		return nil
	}

	// Signal the serve goroutine to exit first
	close(d.done)

	if d.listener != nil {
		_ = d.listener.Close()
		d.listener = nil
	}
	d.started = false

	if d.logger != nil {
		d.logger.Info("DNS server stopped")
	}
	return nil
}

// serve handles incoming DNS queries
func (d *DNSServer) serve(ctx context.Context) {
	buf := make([]byte, 512)
	for {
		select {
		case <-ctx.Done():
			return
		case <-d.done:
			return
		default:
		}

		// Check if listener is still available
		d.mu.RLock()
		listener := d.listener
		d.mu.RUnlock()
		if listener == nil {
			return
		}

		n, remoteAddr, err := listener.ReadFromUDP(buf)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case <-d.done:
				return
			default:
				continue
			}
		}

		// Parse and respond to DNS query
		response := d.handleDNSQuery(buf[:n], remoteAddr)
		if response != nil {
			d.mu.RLock()
			l := d.listener
			d.mu.RUnlock()
			if l != nil {
				_, _ = l.WriteToUDP(response, remoteAddr)
			}
		}
	}
}

// handleDNSQuery processes a raw DNS query and returns a response
// This is a simplified implementation. For production, use miekg/dns library.
func (d *DNSServer) handleDNSQuery(data []byte, remoteAddr *net.UDPAddr) []byte {
	if len(data) < 12 {
		return nil
	}

	// Extract transaction ID (first 2 bytes)
	txID := data[:2]

	// Parse question section (simplified)
	questionCount := int(data[4])<<8 | int(data[5])
	if questionCount == 0 {
		return nil
	}

	// Extract question name (starting at byte 12)
	qname, qtype, _ := parseQuestionName(data, 12)
	if qname == "" {
		return nil
	}

	// Lookup the record
	ip, found := d.Lookup(qname)
	if !found {
		// Return NXDOMAIN response
		return buildNXDOMAIN(txID)
	}

	// Parse query type
	queryType := RecordType(qtype)

	// Build response based on query type
	switch queryType {
	case RecordTypeA:
		return buildARecordResponse(txID, qname, ip, 30)
	case RecordTypeSRV:
		// Try to find port from service record
		port := d.lookupPort(qname)
		if port == 0 {
			port = 80 // default
		}
		return buildSRVRecordResponse(txID, qname, ip, port, 30)
	default:
		return buildNXDOMAIN(txID)
	}
}

func (d *DNSServer) lookupPort(qname string) int {
	d.mu.RLock()
	defer d.mu.RUnlock()

	for _, svc := range d.services {
		fullHostname := fmt.Sprintf("%s.%s.%s", svc.ServiceName, svc.ProjectID, d.domain)
		shortHostname := fmt.Sprintf("%s.%s", svc.ServiceName, d.domain)
		if qname == fullHostname || qname == shortHostname {
			return svc.Port
		}
	}
	return 0
}

func parseQuestionName(data []byte, offset int) (name string, qtype uint16, n int) {
	start := offset
	for offset < len(data) {
		lengthByte := data[offset]
		if lengthByte == 0 {
			offset++
			break
		}
		if offset+int(lengthByte)+1 > len(data) {
			return "", 0, 0
		}
		if offset > start {
			name += "."
		}
		name += string(data[offset+1 : offset+1+int(lengthByte)])
		offset += int(lengthByte) + 1
	}

	if offset+4 > len(data) {
		return "", 0, 0
	}

	qtype = uint16(data[offset])<<8 | uint16(data[offset+1])
	return name, qtype, offset - start + 4
}

func buildARecordResponse(txID []byte, name string, ip string, ttl uint32) []byte {
	var response []byte

	// Header: ID, Flags (response + recursion), QDCount=1, ANCount=1, NSCount=0, ARCount=0
	response = append(response, txID...)
	response = append(response, 0x81, 0x80)      // Flags: response, no error, recursion
	response = append(response, 0x00, 0x01)      // QDCount = 1
	response = append(response, 0x00, 0x01)      // ANCount = 1
	response = append(response, 0x00, 0x00)      // NSCount = 0
	response = append(response, 0x00, 0x00)      // ARCount = 0

	// Question section
	response = appendQuestion(response, name, RecordTypeA)

	// Answer section - use pointer to name
	response = append(response, 0xC0, 0x0C) // Pointer to name at offset 12
	response = append(response, 0x00, 0x01) // Type A
	response = append(response, 0x00, 0x01) // Class IN
	response = append(response, byte(ttl>>24), byte(ttl>>16), byte(ttl>>8), byte(ttl))
	response = append(response, 0x00, 0x04) // RDLength = 4

	// Append IP bytes
	ipBytes := parseIPBytes(ip)
	response = append(response, ipBytes...)

	return response
}

func buildSRVRecordResponse(txID []byte, name string, ip string, port int, ttl uint32) []byte {
	var response []byte

	response = append(response, txID...)
	response = append(response, 0x81, 0x80)
	response = append(response, 0x00, 0x01)
	response = append(response, 0x00, 0x01)
	response = append(response, 0x00, 0x00)
	response = append(response, 0x00, 0x00)

	response = appendQuestion(response, name, RecordTypeSRV)

	response = append(response, 0xC0, 0x0C)
	response = append(response, 0x00, 0x21) // Type SRV
	response = append(response, 0x00, 0x01) // Class IN
	response = append(response, byte(ttl>>24), byte(ttl>>16), byte(ttl>>8), byte(ttl))

	// Simplified SRV response: priority=2 + weight=2 + port=2 + target(1 byte null)
	response = append(response, 0x00, 0x07) // RDLength = 7
	response = append(response, 0x00, 0x00) // Priority = 0
	response = append(response, 0x00, 0x00) // Weight = 0
	response = append(response, byte(port>>8), byte(port))
	response = append(response, 0x00) // Target root

	return response
}

func buildNXDOMAIN(txID []byte) []byte {
	var response []byte
	response = append(response, txID...)
	response = append(response, 0x81, 0x83) // Flags: response, NXDOMAIN
	response = append(response, 0x00, 0x01)
	response = append(response, 0x00, 0x00)
	response = append(response, 0x00, 0x00)
	response = append(response, 0x00, 0x00)
	return response
}

func appendQuestion(b []byte, name string, qtype RecordType) []byte {
	// Encode name
	labels := strings.Split(name, ".")
	for _, label := range labels {
		b = append(b, byte(len(label)))
		b = append(b, label...)
	}
	b = append(b, 0x00) // Null terminator

	b = append(b, byte(qtype>>8), byte(qtype)) // Type
	b = append(b, 0x00, 0x01)                   // Class IN
	return b
}

func parseIPBytes(ip string) []byte {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return []byte{127, 0, 0, 1} // fallback to localhost
	}
	ipv4 := parsed.To4()
	if ipv4 != nil {
		return ipv4
	}
	return []byte{127, 0, 0, 1}
}

func serviceKey(serviceName, projectID string) string {
	return serviceName + "." + projectID
}
