package runtime

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func rewritePostgresHostAuth(path, authMethod string) error {
	authMethod = strings.TrimSpace(authMethod)
	if authMethod == "" {
		return &OperationError{
			Code:      "internal_postgres_invalid_auth_method",
			Message:   "postgres host auth method is required",
			Retryable: false,
		}
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return &OperationError{
			Code:      "internal_postgres_pg_hba_read_failed",
			Message:   fmt.Sprintf("read postgres host auth config %s failed", path),
			Retryable: true,
			Err:       err,
		}
	}
	updated := rewritePostgresHostAuthContent(string(payload), authMethod)
	if updated == string(payload) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return &OperationError{
			Code:      "internal_postgres_pg_hba_dir_failed",
			Message:   fmt.Sprintf("prepare postgres host auth config directory for %s failed", path),
			Retryable: false,
			Err:       err,
		}
	}
	if err := os.WriteFile(path, []byte(updated), 0o600); err != nil {
		return &OperationError{
			Code:      "internal_postgres_pg_hba_write_failed",
			Message:   fmt.Sprintf("write postgres host auth config %s failed", path),
			Retryable: true,
			Err:       err,
		}
	}
	return nil
}

func rewritePostgresHostAuthContent(content, authMethod string) string {
	var builder strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(content))
	rewroteHostLine := false
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			builder.WriteString(line)
			builder.WriteByte('\n')
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 5 && strings.HasPrefix(strings.ToLower(fields[0]), "host") {
			fields[len(fields)-1] = authMethod
			builder.WriteString(strings.Join(fields, " "))
			builder.WriteByte('\n')
			rewroteHostLine = true
			continue
		}
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	if !rewroteHostLine {
		builder.WriteString("host all all all ")
		builder.WriteString(authMethod)
		builder.WriteByte('\n')
	}
	return builder.String()
}

func escapePostgresLiteral(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func escapePostgresIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
