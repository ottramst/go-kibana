// Command coverage compares the endpoints implemented by go-kibana with the
// official Kibana OpenAPI specs and writes COVERAGE.md.
//
// Implemented endpoints are found by scanning the package's Go files for
// annotations on service methods:
//
//	// Kibana API: GET /api/spaces/space
//
// It exits with an error when an annotation matches no operation in any of
// the specs, which catches typos and endpoints Kibana removed.
//
// Run it with `make coverage`, which downloads the specs first.
package main

import (
	"bufio"
	"cmp"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

func main() {
	root := flag.String("root", ".", "repository root")
	specDir := flag.String("specs", ".cache/specs", "directory with kibana-<version>.yaml files, relative to root")
	versions := flag.String("versions", "", "space separated Kibana versions, e.g. \"v8.19.25 v9.5.6\"")
	out := flag.String("out", "COVERAGE.md", "output file, relative to root")
	flag.Parse()

	if err := run(*root, *specDir, strings.Fields(*versions), *out); err != nil {
		fmt.Fprintln(os.Stderr, "coverage:", err)
		os.Exit(1)
	}
}

func run(root, specDir string, versions []string, out string) error {
	if len(versions) == 0 {
		return fmt.Errorf("no versions given")
	}
	specs := make([]*spec, len(versions))
	for i, v := range versions {
		s, err := loadSpec(filepath.Join(root, specDir, "kibana-"+v+".yaml"))
		if err != nil {
			return err
		}
		s.version = v
		specs[i] = s
	}

	impl, err := scanAnnotations(root)
	if err != nil {
		return err
	}

	var unknown []string
	for ep, a := range impl {
		if !slices.ContainsFunc(specs, func(s *spec) bool { _, ok := s.ops[ep]; return ok }) {
			unknown = append(unknown, fmt.Sprintf("%s:%d: %s %s (%s)", a.file, a.line, ep.method, ep.path, a.method))
		}
	}
	if len(unknown) > 0 {
		slices.Sort(unknown)
		return fmt.Errorf("annotated endpoints missing from every spec:\n  %s", strings.Join(unknown, "\n  "))
	}

	f, err := os.Create(filepath.Join(root, out))
	if err != nil {
		return err
	}
	if err := render(f, specs, impl); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

type endpoint struct {
	method string // upper case
	path   string
}

type operation struct {
	tag        string
	deprecated bool
	state      string // x-state, e.g. "Technical Preview; added in 9.1.0"
}

type spec struct {
	version string
	ops     map[endpoint]operation
}

var methods = []string{"get", "post", "put", "patch", "delete"}

func loadSpec(path string) (*spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w (run `make coverage` to download the specs)", err)
	}
	// Path items hold operations keyed by method, next to other fields such
	// as parameters, so decode each method separately.
	var doc struct {
		Paths map[string]map[string]yaml.Node `yaml:"paths"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	s := &spec{ops: make(map[endpoint]operation)}
	for p, item := range doc.Paths {
		for _, m := range methods {
			node, ok := item[m]
			if !ok {
				continue
			}
			var op struct {
				Tags       []string `yaml:"tags"`
				Deprecated bool     `yaml:"deprecated"`
				State      string   `yaml:"x-state"`
			}
			if err := node.Decode(&op); err != nil {
				return nil, fmt.Errorf("parsing %s %s in %s: %w", m, p, path, err)
			}
			tag := "untagged"
			if len(op.Tags) > 0 {
				tag = op.Tags[0]
			}
			s.ops[endpoint{strings.ToUpper(m), p}] = operation{tag: tag, deprecated: op.Deprecated, state: op.State}
		}
	}
	return s, nil
}

type annotation struct {
	file   string
	line   int
	method string // Go method, e.g. SpacesService.List
}

var (
	annotationRE = regexp.MustCompile(`^// Kibana API: (GET|POST|PUT|PATCH|DELETE) (/\S+)$`)
	funcRE       = regexp.MustCompile(`^func \(\w+ \*(\w+)\) (\w+)\(`)
)

// scanAnnotations finds the annotated endpoints in the non-test Go files of
// the root package.
func scanAnnotations(root string) (map[endpoint]annotation, error) {
	files, err := filepath.Glob(filepath.Join(root, "*.go"))
	if err != nil {
		return nil, err
	}
	impl := make(map[endpoint]annotation)
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		if err := scanFile(name, impl); err != nil {
			return nil, err
		}
	}
	return impl, nil
}

func scanFile(name string, impl map[endpoint]annotation) error {
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()

	var pending []endpoint
	var pendingLine int
	sc := bufio.NewScanner(f)
	for line := 1; sc.Scan(); line++ {
		text := sc.Text()
		if m := annotationRE.FindStringSubmatch(text); m != nil {
			pending = append(pending, endpoint{m[1], m[2]})
			pendingLine = line
			continue
		}
		if len(pending) == 0 || strings.HasPrefix(text, "//") {
			continue
		}
		m := funcRE.FindStringSubmatch(text)
		if m == nil {
			return fmt.Errorf("%s:%d: Kibana API annotation is not in a method's doc comment", name, pendingLine)
		}
		for _, ep := range pending {
			if prev, ok := impl[ep]; ok {
				return fmt.Errorf("%s:%d: %s %s is already annotated on %s", name, pendingLine, ep.method, ep.path, prev.method)
			}
			impl[ep] = annotation{file: filepath.Base(name), line: pendingLine, method: m[1] + "." + m[2]}
		}
		pending = nil
	}
	return sc.Err()
}

type row struct {
	endpoint
	op      operation
	in      []bool
	implBy  string
	deprecd bool
}

func render(w io.Writer, specs []*spec, impl map[endpoint]annotation) error {
	rows := make(map[endpoint]*row)
	for i, s := range specs {
		for ep, op := range s.ops {
			r, ok := rows[ep]
			if !ok {
				r = &row{endpoint: ep, in: make([]bool, len(specs))}
				rows[ep] = r
			}
			r.in[i] = true
			// The newest spec wins for the tag and state.
			r.op = op
			r.deprecd = r.deprecd || op.deprecated
		}
	}
	byTag := make(map[string][]*row)
	for ep, r := range rows {
		if a, ok := impl[ep]; ok {
			r.implBy = a.method
		}
		byTag[r.op.tag] = append(byTag[r.op.tag], r)
	}
	tags := make([]string, 0, len(byTag))
	for t, rs := range byTag {
		tags = append(tags, t)
		slices.SortFunc(rs, func(a, b *row) int {
			return cmp.Or(cmp.Compare(a.path, b.path), cmp.Compare(methodOrder(a.method), methodOrder(b.method)))
		})
	}
	slices.SortFunc(tags, func(a, b string) int { return cmp.Compare(strings.ToLower(a), strings.ToLower(b)) })

	versions := make([]string, len(specs))
	for i, s := range specs {
		versions[i] = s.version
	}

	var b strings.Builder
	fmt.Fprintf(&b, "<!-- Code generated by tools/coverage (`make coverage`). DO NOT EDIT. -->\n\n")
	fmt.Fprintf(&b, "# API coverage\n\n")
	fmt.Fprintf(&b, "Endpoints of the official Kibana OpenAPI specs (%s) and the go-kibana methods that implement them. ", strings.Join(versions, ", "))
	fmt.Fprintf(&b, "Planned work is tracked in the [roadmap](https://github.com/ottramst/go-kibana/milestones).\n\n")

	total, done := 0, 0
	for _, r := range rows {
		total++
		if r.implBy != "" {
			done++
		}
	}
	fmt.Fprintf(&b, "**%d of %d endpoints implemented.**\n\n", done, total)

	fmt.Fprintf(&b, "| Area | Implemented |\n|---|---|\n")
	for _, t := range tags {
		n := 0
		for _, r := range byTag[t] {
			if r.implBy != "" {
				n++
			}
		}
		fmt.Fprintf(&b, "| [%s](#%s) | %d / %d |\n", t, anchor(t), n, len(byTag[t]))
	}

	for _, t := range tags {
		fmt.Fprintf(&b, "\n## %s\n\n", t)
		fmt.Fprintf(&b, "| Method | Path | %s | go-kibana | Notes |\n", strings.Join(versions, " | "))
		fmt.Fprintf(&b, "|---|---|%s---|---|\n", strings.Repeat(":-:|", len(versions)))
		for _, r := range byTag[t] {
			avail := make([]string, len(r.in))
			for i, ok := range r.in {
				avail[i] = "—"
				if ok {
					avail[i] = "✅"
				}
			}
			impl := ""
			if r.implBy != "" {
				impl = "`" + r.implBy + "`"
			}
			var notes []string
			if r.deprecd {
				notes = append(notes, "deprecated")
			}
			if r.op.state != "" {
				notes = append(notes, r.op.state)
			}
			fmt.Fprintf(&b, "| %s | `%s` | %s | %s | %s |\n", r.method, r.path, strings.Join(avail, " | "), impl, strings.Join(notes, "; "))
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func methodOrder(m string) int {
	return slices.Index(methods, strings.ToLower(m))
}

// anchor returns the GitHub heading anchor for a heading text.
func anchor(heading string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(heading) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('-')
		}
	}
	return b.String()
}
