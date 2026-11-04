package stacks

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/junaidrehman04/altas/apps/api/internal/security"
)

func TestCatalogListsLaunchKinds(t *testing.T) {
	t.Parallel()
	got := Catalog()
	if len(got) != 3 {
		t.Fatalf("catalog len = %d", len(got))
	}
	want := []string{KindDockerCompose, KindLaravel, KindNextjs}
	seen := map[string]struct{}{}
	for i, item := range got {
		if item.Kind != want[i] {
			t.Fatalf("kind[%d] = %s", i, item.Kind)
		}
		if item.Title == "" || item.Summary == "" || item.Deployable {
			t.Fatalf("definition = %+v", item)
		}
		seen[item.Kind] = struct{}{}
	}
	got[0].Kind = "mutated"
	if Catalog()[0].Kind != KindDockerCompose {
		t.Fatal("catalog copy leaked a mutation")
	}
	if _, ok := Lookup("missing"); ok {
		t.Fatal("missing kind looked up")
	}
	item, ok := Lookup(KindLaravel)
	if !ok || item.Kind != KindLaravel {
		t.Fatalf("lookup = %+v %v", item, ok)
	}
	if len(seen) != 3 {
		t.Fatal("duplicate kinds")
	}
}

func TestProjectPathStaysInJail(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	got, err := ProjectPath(root, "app-1")
	if err != nil {
		t.Fatal(err)
	}
	jail, err := security.OpenJail(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(jail.Root(), "app-1") {
		t.Fatalf("path = %s", got)
	}
	for _, id := range []string{"", "..", "../etc", "a/b", "App", "linked", "app_1", "has space"} {
		if _, err := ProjectPath(root, id); !errors.Is(err, security.ErrEscape) {
			t.Fatalf("ProjectPath(%q) err = %v", id, err)
		}
	}
}
