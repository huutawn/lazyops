package redact

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
)

const maskedValue = "[redacted]"

var (
	bearerPattern  = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/=-]+`)
	kvPattern      = regexp.MustCompile(`(?i)(\b(?:token|access_token|refresh_token|pat|password|secret|api_key|credential|authorization|private_key|kubeconfig|ssh_key|ssh_private_key)\b\s*[:=]\s*)(?:"[^"]*"|'[^']*'|[^\s,]+)`)
	sshKeyPattern  = regexp.MustCompile(`(?i)(-----BEGIN\s+(?:RSA\s+)?PRIVATE\s+KEY-----)[\s\S]*?(-----END\s+(?:RSA\s+)?PRIVATE\s+KEY-----)`)
	sshCertPattern = regexp.MustCompile(`(?i)(ssh-(?:rsa|ed25519|dss)\s+)[A-Za-z0-9+/=]+`)
)

func Text(input string) string {
	output := bearerPattern.ReplaceAllStringFunc(input, func(match string) string {
		parts := strings.Fields(match)
		if len(parts) == 0 {
			return "Bearer " + maskedValue
		}

		return parts[0] + " " + maskedValue
	})

	output = kvPattern.ReplaceAllString(output, "${1}"+maskedValue)
	output = sshKeyPattern.ReplaceAllString(output, "${1} "+maskedValue+" ${2}")
	output = sshCertPattern.ReplaceAllString(output, "${1}"+maskedValue)
	return output
}

func JSON(input []byte) []byte {
	var decoded any
	if err := json.Unmarshal(input, &decoded); err != nil {
		return []byte(Text(string(input)))
	}

	redacted := sanitizeValue(decoded)
	encoded, err := json.Marshal(redacted)
	if err != nil {
		return []byte(Text(string(input)))
	}

	return encoded
}

func PrettyJSON(input []byte) []byte {
	redacted := JSON(input)

	var out bytes.Buffer
	if err := json.Indent(&out, redacted, "", "  "); err != nil {
		return redacted
	}

	return out.Bytes()
}

func sanitizeValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		sanitized := make(map[string]any, len(typed))
		for key, nested := range typed {
			if isSensitiveKey(key) {
				sanitized[key] = maskedValue
				continue
			}

			sanitized[key] = sanitizeValue(nested)
		}
		return sanitized
	case []any:
		sanitized := make([]any, len(typed))
		for idx, nested := range typed {
			sanitized[idx] = sanitizeValue(nested)
		}
		return sanitized
	case string:
		return Text(typed)
	default:
		return value
	}
}

func isSensitiveKey(key string) bool {
	normalized := strings.ToLower(key)
	normalized = strings.ReplaceAll(normalized, "_", "")
	normalized = strings.ReplaceAll(normalized, "-", "")
	normalized = strings.ReplaceAll(normalized, " ", "")

	switch {
	case normalized == "token":
		return true
	case normalized == "pat":
		return true
	case normalized == "password":
		return true
	case normalized == "secret":
		return true
	case normalized == "apikey":
		return true
	case normalized == "credential":
		return true
	case normalized == "authorization":
		return true
	case normalized == "privatekey":
		return true
	case normalized == "kubeconfig":
		return true
	case normalized == "sshkey":
		return true
	case strings.HasSuffix(normalized, "token"):
		return true
	case strings.HasSuffix(normalized, "password"):
		return true
	case strings.HasSuffix(normalized, "secret"):
		return true
	case strings.HasSuffix(normalized, "credential"):
		return true
	case strings.HasSuffix(normalized, "apikey"):
		return true
	case strings.HasSuffix(normalized, "privatekey"):
		return true
	case strings.HasSuffix(normalized, "kubeconfig"):
		return true
	default:
		return false
	}
}
