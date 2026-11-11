package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testSpec = `openapi: 3.0.3
paths:
  /api/things:
    parameters:
      - name: x
        in: query
    get:
      tags: [things]
    post:
      tags: [things]
      deprecated: true
  /api/things/{id}:
    delete:
      tags: [things]
      x-state: Technical Preview; added in 9.1.0
`

func writeRepo(t *testing.T, goSrc string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".cache", "specs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".cache", "specs", "kibana-v1.yaml"), []byte(testSpec), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "things.go"), []byte(goSrc), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRun(t *testing.T) {
	root := writeRepo(t, `package kibana

// List lists things.
//
// Kibana API: GET /api/things
// Kibana API docs: https://example.com
func (s *ThingsService) List() {}
`)
	if err := run(root, ".cache/specs", []string{"v1"}, "COVERAGE.md"); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "COVERAGE.md"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{
		"**1 of 3 endpoints implemented.**",
		"| [things](#things) | 1 / 3 |",
		"| GET | `/api/things` | ✅ | `ThingsService.List` |  |",
		"| POST | `/api/things` | ✅ |  | deprecated |",
		"| DELETE | `/api/things/{id}` | ✅ |  | Technical Preview; added in 9.1.0 |",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("COVERAGE.md does not contain %q:\n%s", want, got)
		}
	}
}

func TestRun_errors(t *testing.T) {
	tests := map[string]string{
		"unknown endpoint": "package kibana\n\n// Kibana API: GET /api/nope\nfunc (s *ThingsService) Nope() {}\n",
		"not on a method":  "package kibana\n\n// Kibana API: GET /api/things\nvar x = 1\n",
		"duplicate": "package kibana\n\n// Kibana API: GET /api/things\nfunc (s *ThingsService) A() {}\n\n" +
			"// Kibana API: GET /api/things\nfunc (s *ThingsService) B() {}\n",
	}
	for name, src := range tests {
		t.Run(name, func(t *testing.T) {
			root := writeRepo(t, src)
			if err := run(root, ".cache/specs", []string{"v1"}, "COVERAGE.md"); err == nil {
				t.Error("run() error = nil, want error")
			}
		})
	}
}

func TestRun_missingSpec(t *testing.T) {
	root := writeRepo(t, "package kibana\n")
	if err := run(root, ".cache/specs", []string{"v2"}, "COVERAGE.md"); err == nil {
		t.Error("run() error = nil, want error")
	}
}
