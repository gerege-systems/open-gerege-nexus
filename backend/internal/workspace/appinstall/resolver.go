package appinstall

import (
	"fmt"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/catalog"
)

type DependencyGraph struct {
	apps map[string]catalog.Manifest
}

func NewDependencyGraph(manifests []catalog.Manifest) *DependencyGraph {
	g := &DependencyGraph{apps: make(map[string]catalog.Manifest)}
	for _, m := range manifests {
		g.apps[m.ID] = m
	}
	return g
}

// ResolveInstallOrder returns the ordered slice of app IDs to install (dependencies first).
// Returns an error if any required dependency is missing or if a cycle is detected.
func (g *DependencyGraph) ResolveInstallOrder(targetAppID string) ([]string, error) {
	visited := make(map[string]bool)
	inStack := make(map[string]bool)
	var order []string

	var visit func(id string) error
	visit = func(id string) error {
		manifest, ok := g.apps[id]
		if !ok {
			return fmt.Errorf("missing dependency: app %q is not available in catalog or binary", id)
		}

		if inStack[id] {
			return fmt.Errorf("dependency cycle detected involving app %q", id)
		}

		if !visited[id] {
			inStack[id] = true
			for _, dep := range manifest.Dependencies {
				if err := visit(dep.ID); err != nil {
					return err
				}
			}
			inStack[id] = false
			visited[id] = true
			order = append(order, id)
		}
		return nil
	}

	if err := visit(targetAppID); err != nil {
		return nil, err
	}

	return order, nil
}
