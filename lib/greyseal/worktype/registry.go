package worktype

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed defaults/*.yaml
var defaultsFS embed.FS

var _ Registry = (*registry)(nil)

type registry struct {
	order []string
	byID  map[string]WorkType
}

// Load builds a Registry from the embedded defaults, then layers every
// *.yaml file in dir on top (a file with a known id replaces that type, a new
// id adds one). An empty dir uses the defaults only; a dir that does not
// exist is an error so a mistyped WORKTYPES_DIR is not silently ignored.
func Load(dir string) (Registry, error) {
	r := &registry{byID: map[string]WorkType{}}

	embedded, err := readFS(defaultsFS, "defaults")
	if err != nil {
		return nil, fmt.Errorf("load embedded work types: %w", err)
	}
	if err := r.add(embedded); err != nil {
		return nil, err
	}

	if dir != "" {
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			return nil, fmt.Errorf("work types dir %s is not a readable directory", dir)
		}
		files, err := readFS(os.DirFS(dir), ".")
		if err != nil {
			return nil, fmt.Errorf("load work types from %s: %w", dir, err)
		}
		if err := r.add(files); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// readFS parses every *.yaml file under root, in file-name order.
func readFS(fsys fs.FS, root string) ([]WorkType, error) {
	paths, err := fs.Glob(fsys, filepath.ToSlash(filepath.Join(root, "*.yaml")))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	var out []WorkType
	for _, p := range paths {
		raw, err := fs.ReadFile(fsys, p)
		if err != nil {
			return nil, err
		}
		var wt WorkType
		dec := yaml.NewDecoder(strings.NewReader(string(raw)))
		dec.KnownFields(true)
		if err := dec.Decode(&wt); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		wt.ID = strings.TrimSpace(wt.ID)
		if wt.ID == "" {
			return nil, fmt.Errorf("%s: id is required", p)
		}
		if strings.TrimSpace(wt.Label) == "" {
			return nil, fmt.Errorf("%s: label is required", p)
		}
		wt.System = strings.ReplaceAll(wt.System, "{{template}}", strings.TrimSpace(wt.Template))
		out = append(out, wt)
	}
	return out, nil
}

// add merges types into the registry. Duplicate ids within one source are
// an error; across sources the later one wins.
func (r *registry) add(types []WorkType) error {
	seen := map[string]bool{}
	for _, wt := range types {
		if seen[wt.ID] {
			return fmt.Errorf("duplicate work type id %q", wt.ID)
		}
		seen[wt.ID] = true
		if _, ok := r.byID[wt.ID]; !ok {
			r.order = append(r.order, wt.ID)
		}
		r.byID[wt.ID] = wt
	}
	return nil
}

func (r *registry) Get(id string) (WorkType, bool) {
	wt, ok := r.byID[id]
	return wt, ok
}

func (r *registry) ByDraftKind(kind string) (WorkType, bool) {
	for _, id := range r.order {
		if wt := r.byID[id]; wt.DraftKind == kind {
			return wt, true
		}
	}
	return WorkType{}, false
}

func (r *registry) Match(reply string) (WorkType, bool) {
	reply = strings.TrimSpace(reply)
	for _, wt := range r.List() {
		if strings.EqualFold(reply, wt.ID) || strings.EqualFold(reply, wt.Label) {
			return wt, true
		}
	}
	return WorkType{}, false
}

func (r *registry) List() []WorkType {
	var out []WorkType
	for _, id := range r.order {
		if wt := r.byID[id]; !wt.Hidden {
			out = append(out, wt)
		}
	}
	return out
}
