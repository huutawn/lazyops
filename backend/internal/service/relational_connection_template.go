package service

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	postgresBasicConnectionTemplateKey = "postgres.basic"
	mysqlBasicConnectionTemplateKey    = "mysql.basic"
)

var (
	relationalConnectionTemplateSlots = []string{
		"DB_URL",
		"DB_NAME",
		"DB_HOST",
		"DB_PORT",
		"DB_USERNAME",
		"DB_PASSWORD",
	}
	relationalConnectionTemplateSlotSet = map[string]struct{}{
		"DB_URL":      {},
		"DB_NAME":     {},
		"DB_HOST":     {},
		"DB_PORT":     {},
		"DB_USERNAME": {},
		"DB_PASSWORD": {},
	}
	relationalConnectionTemplateEnvPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

func defaultConnectionTemplateForKind(kind string) map[string]string {
	if !isRelationalDatabaseKind(kind) {
		return map[string]string{}
	}
	out := make(map[string]string, len(relationalConnectionTemplateSlots))
	for _, slot := range relationalConnectionTemplateSlots {
		out[slot] = slot
	}
	return out
}

func defaultPostgresConnectionTemplate() map[string]string {
	return defaultConnectionTemplateForKind("postgres")
}

func postgresConnectionTemplateForKind(kind string) map[string]string {
	return defaultConnectionTemplateForKind(kind)
}

func normalizeConnectionTemplateForKind(kind string, input map[string]string) (map[string]string, error) {
	template := defaultConnectionTemplateForKind(kind)
	if len(template) == 0 {
		if len(input) > 0 {
			return nil, fmt.Errorf("service.connection_template is only supported for internal postgres or mysql services")
		}
		return map[string]string{}, nil
	}
	if len(input) == 0 {
		return template, nil
	}

	usedEnvNames := make(map[string]string, len(input))
	for rawSlot, rawEnvName := range input {
		slot := strings.ToUpper(strings.TrimSpace(rawSlot))
		if _, ok := relationalConnectionTemplateSlotSet[slot]; !ok {
			return nil, fmt.Errorf("service.connection_template contains unsupported relational slot %q", rawSlot)
		}
		envName := strings.TrimSpace(rawEnvName)
		if envName == "" {
			return nil, fmt.Errorf("service.connection_template[%s] is required", slot)
		}
		if !relationalConnectionTemplateEnvPattern.MatchString(envName) {
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

func normalizePostgresConnectionTemplate(input map[string]string) (map[string]string, error) {
	return normalizeConnectionTemplateForKind("postgres", input)
}

func coerceConnectionTemplateForKind(kind string, raw any) map[string]string {
	switch typed := raw.(type) {
	case map[string]string:
		template, err := normalizeConnectionTemplateForKind(kind, typed)
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
		template, err := normalizeConnectionTemplateForKind(kind, converted)
		if err == nil {
			return template
		}
	}
	return defaultConnectionTemplateForKind(kind)
}

func coercePostgresConnectionTemplate(raw any) map[string]string {
	return coerceConnectionTemplateForKind("postgres", raw)
}

func relationalConnectionTemplateKeyForKind(kind string) string {
	switch normalizeManagedInternalBridgeKind(kind) {
	case "postgres":
		return postgresBasicConnectionTemplateKey
	case "mysql":
		return mysqlBasicConnectionTemplateKey
	default:
		return ""
	}
}

func buildRelationalConnectionRuntimeValues(target ProjectServiceRecord, projectEnv map[string]string, runtimeMode string) map[string]string {
	host := strings.TrimSpace(target.Name)
	if runtimeMode != "distributed-k3s" {
		host = "localhost"
	}
	if host == "" {
		host = "db"
	}
	port := firstPositive(target.ServicePort, target.TargetPort, defaultConfiguredTargetPort(target.Kind))
	targetEnv := cloneStringMap(target.EnvBundle)
	dbKind := normalizeManagedInternalBridgeKind(target.Kind)
	portString := fmt.Sprintf("%d", port)
	switch dbKind {
	case "mysql":
		dbName := firstNonEmptyCompiledValue(targetEnv["MYSQL_DATABASE"], targetEnv["DB_NAME"], projectEnv["MYSQL_DATABASE"], projectEnv["DB_NAME"], "app")
		userName := firstNonEmptyCompiledValue(targetEnv["MYSQL_USER"], targetEnv["DB_USER"], projectEnv["MYSQL_USER"], projectEnv["DB_USER"], "mysql")
		password := firstNonEmptyCompiledValue(targetEnv["MYSQL_PASSWORD"], targetEnv["DB_PASSWORD"], projectEnv["MYSQL_PASSWORD"], projectEnv["DB_PASSWORD"], "mysql")
		return map[string]string{
			"DB_URL":      fmt.Sprintf("mysql://%s:%s@tcp(%s:%s)/%s", userName, password, host, portString, dbName),
			"DB_NAME":     dbName,
			"DB_HOST":     host,
			"DB_PORT":     portString,
			"DB_USERNAME": userName,
			"DB_PASSWORD": password,
		}
	default:
		dbName := firstNonEmptyCompiledValue(targetEnv["POSTGRES_DB"], targetEnv["DB_NAME"], projectEnv["POSTGRES_DB"], projectEnv["DB_NAME"], "app")
		userName := firstNonEmptyCompiledValue(targetEnv["POSTGRES_USER"], targetEnv["DB_USER"], projectEnv["POSTGRES_USER"], projectEnv["DB_USER"], "postgres")
		password := firstNonEmptyCompiledValue(targetEnv["POSTGRES_PASSWORD"], targetEnv["DB_PASSWORD"], projectEnv["POSTGRES_PASSWORD"], projectEnv["DB_PASSWORD"], "postgres")
		return map[string]string{
			"DB_URL":      fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", userName, password, host, portString, dbName),
			"DB_NAME":     dbName,
			"DB_HOST":     host,
			"DB_PORT":     portString,
			"DB_USERNAME": userName,
			"DB_PASSWORD": password,
		}
	}
}

func buildRelationalConnectionTemplateEnv(target ProjectServiceRecord, projectEnv map[string]string, runtimeMode string) map[string]string {
	template := coerceConnectionTemplateForKind(target.Kind, target.ConnectionTemplate)
	values := buildRelationalConnectionRuntimeValues(target, projectEnv, runtimeMode)
	out := make(map[string]string, len(template))
	for _, slot := range relationalConnectionTemplateSlots {
		envName := strings.TrimSpace(template[slot])
		if envName == "" {
			continue
		}
		out[envName] = values[slot]
	}
	return out
}

func buildPostgresConnectionTemplateEnv(target ProjectServiceRecord, projectEnv map[string]string, runtimeMode string) map[string]string {
	return buildRelationalConnectionTemplateEnv(target, projectEnv, runtimeMode)
}

func buildKafkaRuntimeEnv(target ProjectServiceRecord, runtimeMode string) map[string]string {
	host := strings.TrimSpace(target.Name)
	if runtimeMode != "distributed-k3s" {
		host = "localhost"
	}
	if host == "" {
		host = "kafka"
	}
	port := firstPositive(target.ServicePort, target.TargetPort, defaultConfiguredTargetPort(target.Kind))
	if port <= 0 {
		port = 9092
	}
	address := fmt.Sprintf("%s:%d", host, port)
	return map[string]string{
		"KAFKA_BOOTSTRAP_SERVERS": address,
		"KAFKA_BROKERS":           address,
		"KAFKA_HOST":              host,
		"KAFKA_PORT":              fmt.Sprintf("%d", port),
	}
}

func buildMongoRuntimeEnv(target ProjectServiceRecord, runtimeMode string) map[string]string {
	host := strings.TrimSpace(target.Name)
	if runtimeMode != "distributed-k3s" {
		host = "localhost"
	}
	if host == "" {
		host = "mongodb"
	}
	port := firstPositive(target.ServicePort, target.TargetPort, defaultConfiguredTargetPort(target.Kind))
	if port <= 0 {
		port = 27017
	}
	dbName := firstNonEmptyCompiledValue(target.EnvBundle["MONGO_INITDB_DATABASE"], target.EnvBundle["DB_NAME"], "app")
	return map[string]string{
		"MONGODB_HOST": host,
		"MONGODB_PORT": fmt.Sprintf("%d", port),
		"MONGODB_URL":  fmt.Sprintf("mongodb://%s:%d/%s", host, port, dbName),
		"MONGODB_URI":  fmt.Sprintf("mongodb://%s:%d/%s", host, port, dbName),
	}
}

func buildRedisRuntimeEnv(target ProjectServiceRecord, runtimeMode string) map[string]string {
	host := strings.TrimSpace(target.Name)
	if runtimeMode != "distributed-k3s" {
		host = "localhost"
	}
	if host == "" {
		host = "redis"
	}
	port := firstPositive(target.ServicePort, target.TargetPort, defaultConfiguredTargetPort(target.Kind))
	if port <= 0 {
		port = 6379
	}
	return map[string]string{
		"REDIS_HOST": host,
		"REDIS_PORT": fmt.Sprintf("%d", port),
		"REDIS_URL":  fmt.Sprintf("redis://%s:%d/0", host, port),
	}
}

func buildEurekaRuntimeEnv(target ProjectServiceRecord, runtimeMode string) map[string]string {
	host := strings.TrimSpace(target.Name)
	if runtimeMode != "distributed-k3s" {
		host = "localhost"
	}
	if host == "" {
		host = "eureka-server"
	}
	port := firstPositive(target.ServicePort, target.TargetPort, defaultConfiguredTargetPort(target.Kind))
	if port <= 0 {
		port = 8761
	}
	baseURL := fmt.Sprintf("http://%s:%d/eureka", host, port)
	return map[string]string{
		"EUREKA_HOST": host,
		"EUREKA_PORT": fmt.Sprintf("%d", port),
		"EUREKA_URL":  baseURL,
	}
}

func buildDependencyRuntimeEnv(target ProjectServiceRecord, projectEnv map[string]string, runtimeMode string) map[string]string {
	switch normalizeManagedInternalBridgeKind(target.Kind) {
	case "postgres", "mysql":
		return buildRelationalConnectionRuntimeValues(target, projectEnv, runtimeMode)
	case "mongodb":
		return buildMongoRuntimeEnv(target, runtimeMode)
	case "redis":
		return buildRedisRuntimeEnv(target, runtimeMode)
	case "kafka":
		return buildKafkaRuntimeEnv(target, runtimeMode)
	case "eureka-server":
		return buildEurekaRuntimeEnv(target, runtimeMode)
	default:
		return map[string]string{}
	}
}

func buildDependencyBindingEnv(
	binding ProjectServiceDependencyBinding,
	target ProjectServiceRecord,
	projectEnv map[string]string,
	runtimeMode string,
	kindCount int,
	relationalCount int,
) map[string]string {
	kind := normalizeManagedInternalBridgeKind(target.Kind)
	if kind == "" {
		return map[string]string{}
	}
	if isRelationalDatabaseKind(kind) {
		template := cloneStringMap(binding.ConnectionTemplate)
		if len(template) > 0 {
			normalized, err := normalizeConnectionTemplateForKind(kind, template)
			if err == nil {
				template = normalized
			}
		}
		if len(template) == 0 {
			template = cloneStringMap(target.ConnectionTemplate)
		}
		if len(template) == 0 {
			if relationalCount <= 1 && kindCount <= 1 {
				template = defaultConnectionTemplateForKind(kind)
			} else {
				template = prefixedRelationalConnectionTemplate(binding.TargetService)
			}
		}
		values := buildRelationalConnectionRuntimeValues(target, projectEnv, runtimeMode)
		out := make(map[string]string, len(template))
		for _, slot := range relationalConnectionTemplateSlots {
			envName := strings.TrimSpace(template[slot])
			if envName == "" {
				continue
			}
			out[envName] = values[slot]
		}
		return out
	}
	runtimeEnv := buildDependencyRuntimeEnv(target, projectEnv, runtimeMode)
	if len(runtimeEnv) == 0 {
		return runtimeEnv
	}
	if kindCount <= 1 {
		return runtimeEnv
	}
	return prefixDependencyEnv(binding.TargetService, runtimeEnv)
}

func prefixedRelationalConnectionTemplate(alias string) map[string]string {
	prefix := sanitizeProjectEnvAlias(alias)
	if prefix == "" {
		prefix = "DEPENDENCY"
	}
	return map[string]string{
		"DB_URL":      prefix + "_DB_URL",
		"DB_NAME":     prefix + "_DB_NAME",
		"DB_HOST":     prefix + "_DB_HOST",
		"DB_PORT":     prefix + "_DB_PORT",
		"DB_USERNAME": prefix + "_DB_USERNAME",
		"DB_PASSWORD": prefix + "_DB_PASSWORD",
	}
}

func prefixDependencyEnv(alias string, env map[string]string) map[string]string {
	prefix := sanitizeProjectEnvAlias(alias)
	if prefix == "" {
		return env
	}
	out := make(map[string]string, len(env))
	for key, value := range env {
		out[prefix+"_"+key] = value
	}
	return out
}

func isRelationalDatabaseKind(kind string) bool {
	switch normalizeManagedInternalBridgeKind(kind) {
	case "postgres", "mysql":
		return true
	default:
		return false
	}
}
