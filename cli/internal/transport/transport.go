package transport

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

type Request struct {
	Method  string
	Path    string
	Query   map[string]string
	Headers map[string]string
	Body    []byte
}

type Response struct {
	StatusCode  int
	Headers     map[string]string
	Body        []byte
	FixtureName string
}

type Transport interface {
	Do(ctx context.Context, req Request) (Response, error)
	Mode() string
}

func (r Request) Key() string {
	method := strings.ToUpper(strings.TrimSpace(r.Method))
	if method == "" {
		method = "GET"
	}

	return fmt.Sprintf("%s %s", method, buildPath(r.Path, r.Query))
}

func buildPath(path string, query map[string]string) string {
	if len(query) == 0 {
		return path
	}

	values := make(url.Values, len(query))
	keys := make([]string, 0, len(query))
	for key := range query {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		values.Set(key, query[key])
	}

	return path + "?" + values.Encode()
}
