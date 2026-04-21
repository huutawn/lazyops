package service

import (
	"fmt"
	"regexp"
	"strings"
)

const postgresBasicConnectionTemplateKey = "postgres.basic"

var (
	postgresConnectionTemplateSlots = []string{
		"DB_URL",
		"DB_NAME",
		"DB_HOST",
		"DB_PORT",
		"DB_USERNAME",
		"DB_PASSWORD",
	}
	postgresConnectionTemplateSlotSet = map[string]struct{}{
		"DB_URL":      {},
		"DB_NAME":     {},
		"DB_HOST":     {},
		"DB_PORT":     {},
		"DB_USERNAME": {},
		"DB_PASSWORD": {},
	}
	postgresConnectionTemplateEnvPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

func defaultPostgresConnectionTemplate() map[string]string {
	out := make(map[string]string, len(postgresConnectionTemplateSlots))
	for _, slot := range postgresConnectionTemplateSlots {
		out[slot] = slot
	}
	return out
}

func normalizePostgresConnectionTemplate(input map[string]string) (map[string]string, error) {
	template := defaultPostgresConnectionTemplate()
	if len(input) == 0 {
		return template, nil
	}

	usedEnvNames := make(map[string]string, len(input))
	for rawSlot, rawEnvName := range input {
		slot := strings.ToUpper(strings.TrimSpace(rawSlot))
		if _, ok := postgresConnectionTemplateSlotSet[slot]; !ok {
			return nil, fmt.Errorf("service.connection_template contains unsupported postgres slot %q", rawSlot)
		}

		envName := strings.TrimSpace(rawEnvName)
		if envName == "" {
			return nil, fmt.Errorf("service.connection_template[%s] is required", slot)
		}
		if !postgresConnectionTemplateEnvPattern.MatchString(envName) {
			return nil, fmt.Errorf("service.connection_template[%s] must be a valid env var name", slot)
		}
		if existingSlot, exists := usedEnvNames[envName]; exists && existingSlot != slot {
			return nil, fmt.Errorf("service.connection_template maps %s and %s to the same env var %q", existingSlot, slot, envName)
		}

		usedEnvNames[envName] = slot
		template[slot] = envName
	}

	return template, nil
}

func coercePostgresConnectionTemplate(raw any) map[string]string {
	switch typed := raw.(type) {
	case map[string]string:
		template, err := normalizePostgresConnectionTemplate(typed)
		if err == nil {
			return template
		}
	case map[string]any:
		converted := make(map[string]string, len(typed))
		for key, value := range typed {
			if str, ok := value.(string); ok {
				converted[key] = str
			}
		}
		template, err := normalizePostgresConnectionTemplate(converted)
		if err == nil {
			return template
		}
	}

	return defaultPostgresConnectionTemplate()
}

func buildPostgresConnectionRuntimeValues(target ProjectServiceRecord, projectEnv map[string]string, runtimeMode string) map[string]string {
	host := strings.TrimSpace(target.Name)
	if runtimeMode != "distributed-k3s" {
		host = "localhost"
	}
	if host == "" {
		host = "db"
	}

	port := firstPositive(target.ServicePort, target.TargetPort, 5432)
	targetEnv := cloneStringMap(target.EnvBundle)
	dbName := firstNonEmptyCompiledValue(targetEnv["POSTGRES_DB"], targetEnv["DB_NAME"], projectEnv["POSTGRES_DB"], projectEnv["DB_NAME"], "app")
	userName := firstNonEmptyCompiledValue(targetEnv["POSTGRES_USER"], targetEnv["DB_USER"], projectEnv["POSTGRES_USER"], projectEnv["DB_USER"], "postgres")
	password := firstNonEmptyCompiledValue(targetEnv["POSTGRES_PASSWORD"], targetEnv["DB_PASSWORD"], projectEnv["POSTGRES_PASSWORD"], projectEnv["DB_PASSWORD"], "postgres")
	portString := fmt.Sprintf("%d", port)

	return map[string]string{
		"DB_URL":      fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", userName, password, host, portString, dbName),
		"DB_NAME":     dbName,
		"DB_HOST":     host,
		"DB_PORT":     portString,
		"DB_USERNAME": userName,
		"DB_PASSWORD": password,
	}
}

func buildPostgresConnectionTemplateEnv(target ProjectServiceRecord, projectEnv map[string]string, runtimeMode string) map[string]string {
	template := coercePostgresConnectionTemplate(target.ConnectionTemplate)
	values := buildPostgresConnectionRuntimeValues(target, projectEnv, runtimeMode)
	out := make(map[string]string, len(template))
	for _, slot := range postgresConnectionTemplateSlots {
		envName := strings.TrimSpace(template[slot])
		if envName == "" {
			continue
		}
		out[envName] = values[slot]
	}
	return out
}
