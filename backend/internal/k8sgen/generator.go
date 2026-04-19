package k8sgen

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"text/template"
	"time"
)

type DetectedPort struct {
	Port     int
	Protocol string
	Name     string
	Exposed  bool
}

type ServiceSpec struct {
	Name            string
	Kind            string
	Namespace       string
	Public          bool
	PlacementMode   string
	PlacementNodeID string
	ImageRef        string
	ImageDigest     string
	TargetPort      int
	ServicePort     int
	Replicas        int
	Healthcheck     map[string]any
	DetectedPorts   []DetectedPort
	EnvBundle       map[string]string
	PVCSpec         map[string]any
	DeployStrategy  map[string]any
}

type PublicDomain struct {
	ServiceName  string
	PrimaryHost  string
	FallbackHost string
	PrimaryURL   string
	FallbackURL  string
}

type ManifestDocument struct {
	Name    string
	Kind    string
	Path    string
	Content string
}

type ManifestBundle struct {
	Namespace    string
	CombinedYAML string
	RollbackYAML string
	Documents    []ManifestDocument
	GeneratedAt  time.Time
}

type Input struct {
	Namespace     string
	ProjectID     string
	RevisionID    string
	Services      []ServiceSpec
	PublicDomains []PublicDomain
}

type Generator struct {
	now func() time.Time
}

const defaultPlaceholderImageRef = "nginxinc/nginx-unprivileged:stable-alpine"

func NewGenerator() *Generator {
	return &Generator{
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (g *Generator) Generate(input Input) (ManifestBundle, error) {
	namespace := strings.TrimSpace(input.Namespace)
	if namespace == "" {
		return ManifestBundle{}, fmt.Errorf("namespace is required")
	}
	documents := []ManifestDocument{
		{
			Name:    namespace,
			Kind:    "Namespace",
			Path:    "namespace.yaml",
			Content: renderTemplate(namespaceTemplate, map[string]any{"Namespace": namespace}),
		},
	}

	domainIndex := make(map[string]PublicDomain, len(input.PublicDomains))
	for _, item := range input.PublicDomains {
		domainIndex[strings.TrimSpace(item.ServiceName)] = item
	}

	for _, service := range input.Services {
		normalized := normalizeServiceSpec(namespace, service)
		if len(normalized.EnvBundle) > 0 {
			documents = append(documents, ManifestDocument{
				Name:    normalized.Name + "-env",
				Kind:    "Secret",
				Path:    fmt.Sprintf("%s-secret.yaml", normalized.Name),
				Content: renderTemplate(secretTemplate, normalized),
			})
		}

		if normalized.RequiresPVC {
			documents = append(documents, ManifestDocument{
				Name:    normalized.PVCName,
				Kind:    "PersistentVolumeClaim",
				Path:    fmt.Sprintf("%s-pvc.yaml", normalized.Name),
				Content: renderTemplate(pvcTemplate, normalized),
			})
		}

		documents = append(documents,
			ManifestDocument{
				Name:    normalized.Name,
				Kind:    "Service",
				Path:    fmt.Sprintf("%s-service.yaml", normalized.Name),
				Content: renderTemplate(serviceTemplate, normalized),
			},
			ManifestDocument{
				Name:    normalized.Name,
				Kind:    "Deployment",
				Path:    fmt.Sprintf("%s-deployment.yaml", normalized.Name),
				Content: renderTemplate(deploymentTemplate, normalized),
			},
		)

		if normalized.Public {
			if domain, ok := domainIndex[normalized.Name]; ok {
				documents = append(documents, ManifestDocument{
					Name: normalized.Name,
					Kind: "Ingress",
					Path: fmt.Sprintf("%s-ingress.yaml", normalized.Name),
					Content: renderTemplate(ingressTemplate, map[string]any{
						"Service": normalized,
						"Domain":  domain,
					}),
				})
			}
		}
	}

	combined := joinDocuments(documents)
	return ManifestBundle{
		Namespace:    namespace,
		CombinedYAML: combined,
		RollbackYAML: combined,
		Documents:    documents,
		GeneratedAt:  g.now(),
	}, nil
}

type normalizedServiceSpec struct {
	Name            string
	Namespace       string
	Public          bool
	PlacementMode   string
	PlacementNodeID string
	ImageRef        string
	TargetPort      int
	ServicePort     int
	Replicas        int
	EnvBundle       map[string]string
	EnvKeys         []string
	HealthPath      string
	HealthPort      int
	HasHealthCheck  bool
	PVCName         string
	PVCSize         string
	PVCMountPath    string
	RequiresPVC     bool
	Kind            string
}

func normalizeServiceSpec(namespace string, spec ServiceSpec) normalizedServiceSpec {
	targetPort := firstPositive(spec.TargetPort, spec.ServicePort, healthPort(spec.Healthcheck), detectedPrimaryPort(spec.DetectedPorts), defaultPortForKind(spec.Kind))
	servicePort := firstPositive(spec.ServicePort, targetPort)
	replicas := spec.Replicas
	if replicas <= 0 {
		replicas = 1
	}
	env := cloneStringMap(spec.EnvBundle)
	env = normalizeManagedServiceEnv(spec.Kind, env)
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	healthPath := strings.TrimSpace(stringValue(spec.Healthcheck["path"]))
	healthPortValue := firstPositive(intValue(spec.Healthcheck["port"]), targetPort)
	pvcSize := strings.TrimSpace(stringValue(spec.PVCSpec["size"]))
	if pvcSize == "" && requiresPersistentVolume(spec.Kind) {
		pvcSize = "5Gi"
	}

	return normalizedServiceSpec{
		Name:            strings.TrimSpace(spec.Name),
		Namespace:       namespace,
		Public:          spec.Public,
		PlacementMode:   strings.TrimSpace(spec.PlacementMode),
		PlacementNodeID: strings.TrimSpace(spec.PlacementNodeID),
		ImageRef:        firstNonEmpty(spec.ImageRef, defaultPlaceholderImageRef),
		TargetPort:      targetPort,
		ServicePort:     servicePort,
		Replicas:        replicas,
		EnvBundle:       env,
		EnvKeys:         keys,
		HealthPath:      healthPath,
		HealthPort:      healthPortValue,
		HasHealthCheck:  healthPath != "" || healthPortValue > 0,
		PVCName:         strings.TrimSpace(spec.Name) + "-data",
		PVCSize:         pvcSize,
		PVCMountPath:    pvcMountPathForKind(spec.Kind),
		RequiresPVC:     pvcSize != "" || requiresPersistentVolume(spec.Kind),
		Kind:            firstNonEmpty(spec.Kind, "app"),
	}
}

func joinDocuments(docs []ManifestDocument) string {
	if len(docs) == 0 {
		return ""
	}
	parts := make([]string, 0, len(docs))
	for _, doc := range docs {
		if strings.TrimSpace(doc.Content) == "" {
			continue
		}
		parts = append(parts, strings.TrimSpace(doc.Content))
	}
	return strings.Join(parts, "\n---\n")
}

func renderTemplate(source string, data any) string {
	tpl := template.Must(template.New("yaml").Parse(source))
	var buf bytes.Buffer
	_ = tpl.Execute(&buf, data)
	return strings.TrimSpace(buf.String()) + "\n"
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func detectedPrimaryPort(items []DetectedPort) int {
	for _, item := range items {
		if item.Port > 0 {
			return item.Port
		}
	}
	return 0
}

func healthPort(healthcheck map[string]any) int {
	return intValue(healthcheck["port"])
}

func intValue(raw any) int {
	switch value := raw.(type) {
	case int:
		return value
	case int32:
		return int(value)
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}

func stringValue(raw any) string {
	if value, ok := raw.(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func defaultPortForKind(kind string) int {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "postgres":
		return 5432
	case "mysql":
		return 3306
	case "redis":
		return 6379
	case "rabbitmq":
		return 5672
	default:
		return 8080
	}
}

func requiresPersistentVolume(kind string) bool {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "postgres", "mysql", "redis", "rabbitmq":
		return true
	default:
		return false
	}
}

func pvcMountPathForKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "postgres":
		return "/var/lib/postgresql/data"
	case "mysql":
		return "/var/lib/mysql"
	case "redis":
		return "/data"
	case "rabbitmq":
		return "/var/lib/rabbitmq"
	default:
		return "/data"
	}
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func normalizeManagedServiceEnv(kind string, env map[string]string) map[string]string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "postgres":
		if env == nil {
			env = map[string]string{}
		}
		if strings.TrimSpace(env["POSTGRES_DB"]) == "" {
			env["POSTGRES_DB"] = "app"
		}
		if strings.TrimSpace(env["POSTGRES_USER"]) == "" {
			env["POSTGRES_USER"] = "postgres"
		}
		if strings.TrimSpace(env["POSTGRES_PASSWORD"]) == "" {
			env["POSTGRES_PASSWORD"] = firstNonEmpty(env["DB_PASSWORD"], "postgres")
		}
	}
	return env
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

const namespaceTemplate = `
apiVersion: v1
kind: Namespace
metadata:
  name: {{ .Namespace }}
`

const secretTemplate = `
apiVersion: v1
kind: Secret
metadata:
  name: {{ .Name }}-env
  namespace: {{ .Namespace }}
type: Opaque
stringData:
{{- range .EnvKeys }}
  {{ . }}: {{ index $.EnvBundle . | printf "%q" }}
{{- end }}
`

const pvcTemplate = `
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: {{ .PVCName }}
  namespace: {{ .Namespace }}
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: {{ .PVCSize }}
`

const serviceTemplate = `
apiVersion: v1
kind: Service
metadata:
  name: {{ .Name }}
  namespace: {{ .Namespace }}
spec:
  type: ClusterIP
  selector:
    app.kubernetes.io/name: {{ .Name }}
  ports:
    - name: http
      port: {{ .ServicePort }}
      targetPort: {{ .TargetPort }}
      protocol: TCP
`

const deploymentTemplate = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Name }}
  namespace: {{ .Namespace }}
spec:
  replicas: {{ .Replicas }}
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 0
      maxSurge: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: {{ .Name }}
  template:
    metadata:
      labels:
        app.kubernetes.io/name: {{ .Name }}
        lazyops.service: {{ .Name }}
    spec:
{{- if and (eq .PlacementMode "pinned_node") .PlacementNodeID }}
      nodeSelector:
        lazyops.io/instance-id: {{ .PlacementNodeID }}
{{- end }}
      containers:
        - name: {{ .Name }}
          image: {{ .ImageRef }}
          imagePullPolicy: IfNotPresent
          ports:
            - containerPort: {{ .TargetPort }}
              protocol: TCP
{{- if .EnvKeys }}
          envFrom:
            - secretRef:
                name: {{ .Name }}-env
{{- end }}
{{- if .HasHealthCheck }}
          readinessProbe:
{{- if .HealthPath }}
            httpGet:
              path: {{ .HealthPath }}
              port: {{ .HealthPort }}
{{- else }}
            tcpSocket:
              port: {{ .HealthPort }}
{{- end }}
            initialDelaySeconds: 5
            periodSeconds: 10
          livenessProbe:
{{- if .HealthPath }}
            httpGet:
              path: {{ .HealthPath }}
              port: {{ .HealthPort }}
{{- else }}
            tcpSocket:
              port: {{ .HealthPort }}
{{- end }}
            initialDelaySeconds: 15
            periodSeconds: 20
{{- end }}
{{- if .RequiresPVC }}
          volumeMounts:
            - name: {{ .PVCName }}
              mountPath: {{ .PVCMountPath }}
      volumes:
        - name: {{ .PVCName }}
          persistentVolumeClaim:
            claimName: {{ .PVCName }}
{{- end }}
`

const ingressTemplate = `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{ .Service.Name }}
  namespace: {{ .Service.Namespace }}
  annotations:
    kubernetes.io/ingress.class: traefik
spec:
  ingressClassName: traefik
  rules:
    - host: {{ .Domain.FallbackHost }}
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: {{ .Service.Name }}
                port:
                  number: {{ .Service.ServicePort }}
`
