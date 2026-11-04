// Package stacks is the registry of project templates. This release lists
// kinds and can inventory local Compose projects. It does not create,
// start, or stop containers.
package stacks

import (
	"errors"
	"regexp"

	"github.com/junaidrehman04/altas/apps/api/internal/security"
)

const (
	KindDockerCompose = "docker-compose"
	KindLaravel       = "laravel"
	KindNextjs        = "nextjs"
)

// Definition describes one stack kind. Deployable stays false until a later
// release can create a project from a template.
type Definition struct {
	Kind       string `json:"kind"`
	Title      string `json:"title"`
	Summary    string `json:"summary"`
	Deployable bool   `json:"deployable"`
}

// ErrUnknownKind means the kind is not in the registry.
var ErrUnknownKind = errors.New("unknown stack kind")

var projectID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

var catalog = []Definition{
	{
		Kind:       KindDockerCompose,
		Title:      "Docker Compose",
		Summary:    "Compose project template. A read-only inventory is available when the Docker probe is enabled. Deploy is not available yet.",
		Deployable: false,
	},
	{
		Kind:       KindLaravel,
		Title:      "Laravel",
		Summary:    "Laravel application stack. Registered for a later template deploy.",
		Deployable: false,
	},
	{
		Kind:       KindNextjs,
		Title:      "Next.js",
		Summary:    "Next.js application stack. Registered for a later template deploy.",
		Deployable: false,
	},
}

// Catalog returns a copy of the known stack kinds.
func Catalog() []Definition {
	out := make([]Definition, len(catalog))
	copy(out, catalog)
	return out
}

// Lookup finds one kind. The ok result is false when kind is not registered.
func Lookup(kind string) (Definition, bool) {
	for _, item := range catalog {
		if item.Kind == kind {
			return item, true
		}
	}
	return Definition{}, false
}

// ProjectPath resolves id under root using the path jail.
// id is a single path segment. The directory does not have to exist yet.
func ProjectPath(root, id string) (string, error) {
	if !projectID.MatchString(id) || id == "" {
		return "", security.ErrEscape
	}
	jail, err := security.OpenJail(root)
	if err != nil {
		return "", err
	}
	return jail.Resolve(id)
}
