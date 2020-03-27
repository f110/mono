package consumer

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestGitRepo_modifyKustomization(t *testing.T) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	err = ioutil.WriteFile(filepath.Join(dir, "kustomization.yaml"), []byte(`namespace: bot

images:
  - name: registry.f110.dev/discord-bot/bot
    digest: sha256:a1dfef369a86d399f7445c8ba3c3dffa1079f731120a886dc26d2e9bf9dcc402 # bot:registry.f110.dev/discord-bot/bot
  - name: registry.f110.dev/discord-bot/sidecar
    digest: sha256:oldhash # bot:registry.f110.dev/discord-bot/sidecar`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	g := &gitRepo{dir: dir, image: "registry.f110.dev/discord-bot/bot"}
	editedFiles, err := g.modifyKustomization([]string{"kustomization.yaml"}, "sha256:newhash")
	if len(editedFiles) == 0 {
		t.Fatal("Expect edit file but not")
	}

	b, err := ioutil.ReadFile(filepath.Join(dir, "kustomization.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	expectResult := `namespace: bot

images:
  - name: registry.f110.dev/discord-bot/bot
    digest: sha256:newhash # bot:registry.f110.dev/discord-bot/bot
  - name: registry.f110.dev/discord-bot/sidecar
    digest: sha256:oldhash # bot:registry.f110.dev/discord-bot/sidecar`
	if string(b) != expectResult {
		t.Log(string(b))
		t.Fatal("unexpected file modification")
	}
}
