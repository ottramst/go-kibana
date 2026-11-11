// Command coverage compares the endpoints implemented by go-kibana with the
// official Kibana OpenAPI specs and writes COVERAGE.md.
//
// Implemented endpoints are found by scanning the package's Go files for
// annotations on service methods:
//
//	// Kibana API: GET /api/spaces/space
//
// It exits with an error when an annotation is malformed, is not in a
// method's doc comment, is duplicated, or matches no operation in any of the
// specs. The last catches typos, and endpoints removed from every supported
// version.
//
// Versions are listed oldest first: for an endpoint in several specs, the
// newest spec's tag and x-state are shown.
//
// The specs are the Kibana OpenAPI specification by Elastic, licensed under
// CC BY-NC-ND 4.0. They are downloaded at build time and never committed;
// COVERAGE.md only lists their operations' methods, paths, tags and states.
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
	versions := flag.String("versions", "", "space separated Kibana versions, oldest first, e.g. \"v8.19.25 v9.5.6\"")
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
			tag := ""
			if len(op.Tags) > 0 {
				tag = op.Tags[0]
			}
			s.ops[endpoint{strings.ToUpper(m), p}] = operation{tag: tag, deprecated: op.Deprecated, state: op.State}
		}
	}
	inferTags(s.ops)
	return s, nil
}

// inferTags gives untagged operations the tag of the tagged operation whose
// path shares the most leading segments with theirs, such as "roles" for
// POST /api/security/role/_query.
func inferTags(ops map[endpoint]operation) {
	for ep, op := range ops {
		if op.tag != "" {
			continue
		}
		best, bestLen := "untagged", 1 // require more than the shared "/api"
		for other, o := range ops {
			if o.tag == "" {
				continue
			}
			n := commonSegments(ep.path, other.path)
			if n > bestLen || n == bestLen && best != "untagged" && o.tag < best {
				best, bestLen = o.tag, n
			}
		}
		op.tag = best
		ops[ep] = op
	}
}

func commonSegments(a, b string) int {
	as, bs := strings.Split(strings.Trim(a, "/"), "/"), strings.Split(strings.Trim(b, "/"), "/")
	n := 0
	for n < len(as) && n < len(bs) && as[n] == bs[n] {
		n++
	}
	return n
}

type annotation struct {
	file   string
	line   int
	method string // Go method, e.g. SpacesService.List
}

var (
	annotationPrefixRE = regexp.MustCompile(`^\s*//\s*Kibana API:`)
	annotationRE       = regexp.MustCompile(`^// Kibana API: (GET|POST|PUT|PATCH|DELETE) (/\S+)$`)
	// funcRE matches method declarations with named, unnamed, pointer, value
	// and generic receivers.
	funcRE = regexp.MustCompile(`^func \((?:\w+ )?\*?(\w+)(?:\[[^\]]*\])?\) (\w+)[\[(]`)
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

	type pendingAnnotation struct {
		endpoint
		line int
	}
	var pending []pendingAnnotation
	sc := bufio.NewScanner(f)
	for line := 1; sc.Scan(); line++ {
		text := sc.Text()
		if annotationPrefixRE.MatchString(text) {
			// "Kibana API docs:" lines are links, not annotations.
			if strings.Contains(text, "Kibana API docs:") {
				continue
			}
			m := annotationRE.FindStringSubmatch(text)
			if m == nil {
				return fmt.Errorf("%s:%d: malformed annotation %q, want \"// Kibana API: METHOD /path\" with an upper case method", name, line, strings.TrimSpace(text))
			}
			pending = append(pending, pendingAnnotation{endpoint{m[1], m[2]}, line})
			continue
		}
		if len(pending) == 0 || strings.HasPrefix(text, "//") {
			continue
		}
		m := funcRE.FindStringSubmatch(text)
		if m == nil {
			return fmt.Errorf("%s:%d: Kibana API annotation is not in a method's doc comment", name, pending[0].line)
		}
		for _, p := range pending {
			if prev, ok := impl[p.endpoint]; ok {
				return fmt.Errorf("%s:%d: %s %s is already annotated on %s", name, p.line, p.method, p.path, prev.method)
			}
			impl[p.endpoint] = annotation{file: filepath.Base(name), line: p.line, method: m[1] + "." + m[2]}
		}
		pending = nil
	}
	if err := sc.Err(); err != nil {
		return err
	}
	if len(pending) > 0 {
		return fmt.Errorf("%s:%d: Kibana API annotation is not in a method's doc comment", name, pending[0].line)
	}
	return nil
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
	slices.SortFunc(tags, func(a, b string) int {
		return cmp.Or(cmp.Compare(strings.ToLower(a), strings.ToLower(b)), cmp.Compare(a, b))
	})
	anchors := headingAnchors(tags)

	versions := make([]string, len(specs))
	for i, s := range specs {
		versions[i] = s.version
	}

	var b strings.Builder
	fmt.Fprintf(&b, "<!-- Code generated by tools/coverage (`make coverage`). DO NOT EDIT. -->\n\n")
	fmt.Fprintf(&b, "# API coverage\n\n")
	fmt.Fprintf(&b, "Endpoints of the official Kibana OpenAPI specs (%s) and the go-kibana methods that implement them. ", strings.Join(versions, ", "))
	fmt.Fprintf(&b, "Planned work is tracked in the [roadmap](https://github.com/ottramst/go-kibana/milestones).\n\n")
	fmt.Fprintf(&b, "Endpoint data comes from the [Kibana OpenAPI specification](https://github.com/elastic/kibana/tree/main/oas_docs) by Elastic, licensed under [CC BY-NC-ND 4.0](https://creativecommons.org/licenses/by-nc-nd/4.0/).\n\n")

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
		fmt.Fprintf(&b, "| [%s](#%s) | %d / %d |\n", cell(t), anchors[t], n, len(byTag[t]))
	}

	for _, t := range tags {
		fmt.Fprintf(&b, "\n## %s\n\n", cell(t))
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
			fmt.Fprintf(&b, "| %s | `%s` | %s | %s | %s |\n", r.method, cell(r.path), strings.Join(avail, " | "), impl, cell(strings.Join(notes, "; ")))
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func methodOrder(m string) int {
	return slices.Index(methods, strings.ToLower(m))
}

// cell makes text safe for a Markdown table cell or heading.
func cell(s string) string {
	s = strings.ReplaceAll(s, "|", `\|`)
	return strings.Join(strings.Fields(s), " ")
}

// headingAnchors returns the GitHub anchors of the headings, in order:
// repeated anchors get a "-1", "-2", ... suffix like GitHub adds.
func headingAnchors(headings []string) map[string]string {
	anchors := make(map[string]string, len(headings))
	seen := make(map[string]int)
	for _, h := range headings {
		a := anchor(cell(h))
		if n := seen[a]; n > 0 {
			seen[a]++
			a = fmt.Sprintf("%s-%d", a, n)
		} else {
			seen[a] = 1
		}
		anchors[h] = a
	}
	return anchors
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
