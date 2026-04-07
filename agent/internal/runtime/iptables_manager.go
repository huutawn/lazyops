package runtime

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

const (
	// iptablesChain is the custom chain used by LazyOps sidecar for DNAT rules.
	iptablesChain = "LAZYOPS_SIDECAR"
	// iptablesTable is the netfilter table used for port redirection.
	iptablesTable = "nat"
)

// IPTablesRule represents a single DNAT redirect rule.
type IPTablesRule struct {
	Alias        string `json:"alias"`
	Protocol     string `json:"protocol"`
	OriginalPort int    `json:"original_port"`
	RedirectPort int    `json:"redirect_port"`
	Comment      string `json:"comment,omitempty"`
}

// IPTablesManager manages iptables DNAT rules for localhost_rescue mode.
// It creates a dedicated LAZYOPS_SIDECAR chain in the nat table to avoid
// interfering with other iptables rules on the system.
type IPTablesManager struct {
	logger    *slog.Logger
	mu        sync.Mutex
	rules     map[string]IPTablesRule
	available bool
	checked   bool
	execFunc  func(args ...string) ([]byte, error)
}

// NewIPTablesManager creates a new iptables manager.
func NewIPTablesManager(logger *slog.Logger) *IPTablesManager {
	return &IPTablesManager{
		logger: logger,
		rules:  make(map[string]IPTablesRule),
		execFunc: func(args ...string) ([]byte, error) {
			cmd := exec.Command("iptables", args...)
			return cmd.CombinedOutput()
		},
	}
}

// WithExecFunc allows overriding the iptables execution function (for testing).
func (m *IPTablesManager) WithExecFunc(fn func(args ...string) ([]byte, error)) *IPTablesManager {
	m.execFunc = fn
	return m
}

// EnsureChain creates the LAZYOPS_SIDECAR chain if it doesn't exist,
// and inserts a jump rule from OUTPUT to this chain.
func (m *IPTablesManager) EnsureChain() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.isAvailable() {
		return nil
	}

	// Create chain (ignore "already exists" error)
	output, err := m.execFunc("-t", iptablesTable, "-N", iptablesChain)
	if err != nil && !strings.Contains(string(output), "already exists") {
		return fmt.Errorf("failed to create iptables chain %s: %s: %w", iptablesChain, string(output), err)
	}

	// Check if jump rule already exists
	_, err = m.execFunc("-t", iptablesTable, "-C", "OUTPUT", "-j", iptablesChain)
	if err != nil {
		// Insert jump rule at the top of OUTPUT
		output, err = m.execFunc("-t", iptablesTable, "-I", "OUTPUT", "1", "-j", iptablesChain)
		if err != nil {
			return fmt.Errorf("failed to insert jump rule to %s: %s: %w", iptablesChain, string(output), err)
		}
	}

	if m.logger != nil {
		m.logger.Info("iptables chain ensured", "chain", iptablesChain, "table", iptablesTable)
	}

	return nil
}

// AddDNATRule adds a DNAT redirect rule so that traffic to localhost:originalPort
// is redirected to localhost:redirectPort (where the sidecar proxy listens).
func (m *IPTablesManager) AddDNATRule(rule IPTablesRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.isAvailable() {
		if m.logger != nil {
			m.logger.Warn("iptables not available, skipping DNAT rule",
				"alias", rule.Alias,
				"original_port", rule.OriginalPort,
				"redirect_port", rule.RedirectPort,
			)
		}
		m.rules[ruleKey(rule)] = rule
		return nil
	}

	comment := rule.Comment
	if comment == "" {
		comment = fmt.Sprintf("lazyops-sidecar-%s", rule.Alias)
	}

	protocol := rule.Protocol
	if protocol == "http" {
		protocol = "tcp"
	}

	args := []string{
		"-t", iptablesTable,
		"-A", iptablesChain,
		"-p", protocol,
		"-d", "127.0.0.1",
		"--dport", strconv.Itoa(rule.OriginalPort),
		"-j", "REDIRECT",
		"--to-port", strconv.Itoa(rule.RedirectPort),
		"-m", "comment", "--comment", comment,
	}

	output, err := m.execFunc(args...)
	if err != nil {
		return fmt.Errorf("failed to add DNAT rule for port %d→%d: %s: %w",
			rule.OriginalPort, rule.RedirectPort, strings.TrimSpace(string(output)), err)
	}

	m.rules[ruleKey(rule)] = rule

	if m.logger != nil {
		m.logger.Info("iptables DNAT rule added",
			"alias", rule.Alias,
			"original_port", rule.OriginalPort,
			"redirect_port", rule.RedirectPort,
			"comment", comment,
		)
	}

	return nil
}

// RemoveDNATRule removes a previously added DNAT rule.
func (m *IPTablesManager) RemoveDNATRule(rule IPTablesRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.rules, ruleKey(rule))

	if !m.isAvailable() {
		return nil
	}

	comment := rule.Comment
	if comment == "" {
		comment = fmt.Sprintf("lazyops-sidecar-%s", rule.Alias)
	}

	protocol := rule.Protocol
	if protocol == "http" {
		protocol = "tcp"
	}

	args := []string{
		"-t", iptablesTable,
		"-D", iptablesChain,
		"-p", protocol,
		"-d", "127.0.0.1",
		"--dport", strconv.Itoa(rule.OriginalPort),
		"-j", "REDIRECT",
		"--to-port", strconv.Itoa(rule.RedirectPort),
		"-m", "comment", "--comment", comment,
	}

	output, err := m.execFunc(args...)
	if err != nil {
		// Rule might not exist, log warning but don't fail
		if m.logger != nil {
			m.logger.Warn("failed to remove DNAT rule (may not exist)",
				"alias", rule.Alias,
				"original_port", rule.OriginalPort,
				"redirect_port", rule.RedirectPort,
				"output", strings.TrimSpace(string(output)),
			)
		}
		return nil
	}

	if m.logger != nil {
		m.logger.Info("iptables DNAT rule removed",
			"alias", rule.Alias,
			"original_port", rule.OriginalPort,
			"redirect_port", rule.RedirectPort,
		)
	}

	return nil
}

// CleanupAll flushes the LAZYOPS_SIDECAR chain, removes the jump rule from
// OUTPUT, and deletes the chain.
func (m *IPTablesManager) CleanupAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.rules = make(map[string]IPTablesRule)

	if !m.isAvailable() {
		return nil
	}

	// Flush the custom chain
	_, _ = m.execFunc("-t", iptablesTable, "-F", iptablesChain)

	// Remove jump rule from OUTPUT
	_, _ = m.execFunc("-t", iptablesTable, "-D", "OUTPUT", "-j", iptablesChain)

	// Delete the custom chain
	output, err := m.execFunc("-t", iptablesTable, "-X", iptablesChain)
	if err != nil && !strings.Contains(string(output), "No chain") {
		return fmt.Errorf("failed to delete iptables chain %s: %s: %w", iptablesChain, string(output), err)
	}

	if m.logger != nil {
		m.logger.Info("iptables LAZYOPS_SIDECAR chain cleaned up")
	}

	return nil
}

// ListRules returns all currently tracked DNAT rules.
func (m *IPTablesManager) ListRules() []IPTablesRule {
	m.mu.Lock()
	defer m.mu.Unlock()

	rules := make([]IPTablesRule, 0, len(m.rules))
	for _, rule := range m.rules {
		rules = append(rules, rule)
	}
	return rules
}

// IsAvailable returns whether iptables is available on the system.
func (m *IPTablesManager) IsAvailable() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.isAvailable()
}

func (m *IPTablesManager) isAvailable() bool {
	if m.checked {
		return m.available
	}

	m.checked = true
	_, err := exec.LookPath("iptables")
	m.available = err == nil

	if !m.available && m.logger != nil {
		m.logger.Warn("iptables binary not found, DNAT rules will be skipped (dev mode)")
	}

	return m.available
}

func ruleKey(rule IPTablesRule) string {
	return fmt.Sprintf("%s:%d:%d", rule.Alias, rule.OriginalPort, rule.RedirectPort)
}
