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
    put:
      tags: [things]
      x-state: "a | b"
  /api/things/_query:
    post: {}
  /api/other:
    get:
      tags: [Things]
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

func TestRun_separatedAnnotations(t *testing.T) {
	// Blank comment lines between the annotations, so that godoc renders
	// them as separate paragraphs, and two annotations on one method.
	root := writeRepo(t, `package kibana

// Create creates a thing.
//
// Kibana API: POST /api/things
//
// Kibana API: PUT /api/things/{id}
//
// Kibana API docs: https://example.com
func (s *ThingsService) Create() {}
`)
	if err := run(root, ".cache/specs", []string{"v1"}, "COVERAGE.md"); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "COVERAGE.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"| POST | `/api/things` | ✅ | `ThingsService.Create` | deprecated |",
		"| PUT | `/api/things/{id}` | ✅ | `ThingsService.Create` | a \\| b |",
	} {
		if !strings.Contains(string(data), want) {
			t.Errorf("COVERAGE.md does not contain %q:\n%s", want, data)
		}
	}
}

func TestRun(t *testing.T) {
	root := writeRepo(t, `package kibana

// List lists things.
//
// Kibana API: GET /api/things
// Kibana API docs: https://example.com
func (s *ThingsService) List() {}

// Query queries things; a value receiver.
//
// Kibana API: POST /api/things/_query
func (s ThingsService) Query() {}

// Generic receiver.
//
// Kibana API: GET /api/other
func (s *Others[T]) Get() {}
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
		"**3 of 6 endpoints implemented.**",
		// Tags that differ only in case get distinct anchors, like on GitHub.
		"| [Things](#things) | 1 / 1 |",
		"| [things](#things-1) | 2 / 5 |",
		// The untagged operation gets the tag of its closest path.
		"| POST | `/api/things/_query` | ✅ | `ThingsService.Query` |  |",
		"| GET | `/api/other` | ✅ | `Others.Get` |  |",
		"| PUT | `/api/things/{id}` | ✅ |  | a \\| b |",
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
		"lower case method":  "package kibana\n\n// Kibana API: get /api/things\nfunc (s *ThingsService) A() {}\n",
		"mixed case method":  "package kibana\n\n// Kibana API: Get /api/things\nfunc (s *ThingsService) A() {}\n",
		"trailing space":     "package kibana\n\n// Kibana API: GET /api/things \nfunc (s *ThingsService) A() {}\n",
		"unsupported method": "package kibana\n\n// Kibana API: HEAD /api/things\nfunc (s *ThingsService) A() {}\n",
		"indented":           "package kibana\n\nfunc f() {\n\t// Kibana API: GET /api/things\n}\n",
		"end of file":        "package kibana\n\n// Kibana API: GET /api/things\n",
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
