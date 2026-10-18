package vault

import "reflect"

// Resolve returns true to keep the local change, false to keep the remote change.
// Values are deliberately excluded from conflict labels.
type Resolve func(path string) (bool, error)

func Clone(v *Vault) *Vault {
	out := New()
	out.Version = v.Version
	for p, project := range v.Projects {
		services := map[string]Service{}
		for s, service := range project.Services {
			fields := map[string]Field{}
			for f, field := range service.Fields {
				fields[f] = field
			}
			services[s] = Service{Fields: fields}
		}
		out.Projects[p] = Project{Services: services}
	}
	return out
}

// Reconcile performs a three-way merge, preserving empty projects/services and
// treating deletion versus descendant changes as a conflict at the deleted node.
func Reconcile(base, local, remote *Vault, resolve Resolve) (*Vault, error) {
	projects, err := mergeMap(base.Projects, local.Projects, remote.Projects, "", resolve, func(b, l, r Project, path string) (Project, error) {
		services, err := mergeMap(b.Services, l.Services, r.Services, path, resolve, func(b, l, r Service, path string) (Service, error) {
			fields, err := mergeMap(b.Fields, l.Fields, r.Fields, path, resolve, nil)
			return Service{Fields: fields}, err
		})
		return Project{Services: services}, err
	})
	if err != nil {
		return nil, err
	}
	out := &Vault{Version: currentVersion, Projects: projects}
	// Reject case-only collisions introduced by independent additions.
	for _, p := range sortedKeys(projects) {
		if e, ok := caseConflict(mapKeys(projects), p); ok {
			return nil, &CaseConflictError{Kind: "project", Existing: e, Given: p}
		}
		for s, service := range projects[p].Services {
			if e, ok := caseConflict(mapKeys(projects[p].Services), s); ok {
				return nil, &CaseConflictError{Kind: "service", Existing: e, Given: s}
			}
			for f := range service.Fields {
				if e, ok := caseConflict(mapKeys(service.Fields), f); ok {
					return nil, &CaseConflictError{Kind: "field", Existing: e, Given: f}
				}
			}
		}
	}
	return out, nil
}

func mergeMap[T any](base, local, remote map[string]T, path string, resolve Resolve, descend func(T, T, T, string) (T, error)) (map[string]T, error) {
	out := map[string]T{}
	keys := map[string]bool{}
	for k := range base {
		keys[k] = true
	}
	for k := range local {
		keys[k] = true
	}
	for k := range remote {
		keys[k] = true
	}
	for _, k := range sortedKeys(keys) {
		b, bok := base[k]
		l, lok := local[k]
		r, rok := remote[k]
		equal := func(a T, aok bool, b T, bok bool) bool { return aok == bok && (!aok || reflect.DeepEqual(a, b)) }
		label := k
		if path != "" {
			label = path + "/" + k
		}
		switch {
		case equal(l, lok, b, bok):
			if rok {
				out[k] = r
			}
		case equal(r, rok, b, bok), equal(l, lok, r, rok):
			if lok {
				out[k] = l
			}
		case lok && rok && descend != nil:
			merged, err := descend(b, l, r, label)
			if err != nil {
				return nil, err
			}
			out[k] = merged
		default:
			mine, err := resolve(label)
			if err != nil {
				return nil, err
			}
			if mine {
				if lok {
					out[k] = l
				}
			} else if rok {
				out[k] = r
			}
		}
	}
	return out, nil
}
