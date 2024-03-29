package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-config-inspect/tfconfig"
	"github.com/samber/lo"
)

var (
	flagJSON = flag.Bool("json", false, "output JSON")
)

func main() {
	flag.Parse()

	var dir string
	if flag.NArg() > 0 {
		dir = flag.Arg(0)
	} else {
		dir = "."
	}

	if !tfconfig.IsModuleDir(dir) {
		fmt.Fprintf(os.Stderr, "error: %s is not a Terraform module directory\n", dir)
		os.Exit(1)
	}

	m, err := calcModuleHash("root", dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error calculating module hash: %s\n", err)
		os.Exit(1)
	}

	if *flagJSON {
		showJSON(m)
	} else {
		os.Stdout.Write([]byte(m.Hash()))
		os.Stdout.Write([]byte{'\n'})
	}
}

type ModuleInfo struct {
	Name  string        `json:"name"`
	Files []*FileInfo   `json:"files"`
	Deps  []*ModuleInfo `json:"deps"`
}

type FileInfo struct {
	Name string `json:"name"`
	Hash string `json:"hash"`
}

func (m ModuleInfo) Hash() string {
	raw := ""

	raw += strings.Join(lo.Map(m.Files, func(f *FileInfo, _ int) string {
		return f.Hash
	}), " ")

	raw += strings.Join(lo.Map(m.Deps, func(d *ModuleInfo, _ int) string {
		return d.Hash()
	}), " ")

	h := hasher()
	_, _ = io.WriteString(h, raw)
	return hex.EncodeToString(h.Sum(nil))
}

func showJSON(module *ModuleInfo) {
	j, err := json.MarshalIndent(module, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error producing JSON: %s\n", err)
		os.Exit(2)
	}
	os.Stdout.Write(j)
	os.Stdout.Write([]byte{'\n'})
}

func calcModuleHash(name string, dir string) (*ModuleInfo, error) {
	m, diag := tfconfig.LoadModule(dir)
	if err := diag.Err(); err != nil {
		return nil, err
	}

	// list files
	fileHashes := make([]*FileInfo, 0)
	files, err := os.ReadDir(m.Path)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".tf" {
			continue
		}
		h, err := calcFileHash(filepath.Join(m.Path, file.Name()))
		if err != nil {
			return nil, err
		}
		fileHashes = append(fileHashes, &FileInfo{
			Name: file.Name(),
			Hash: h,
		})
	}

	deps := make([]*ModuleInfo, 0)
	for k, mc := range m.ModuleCalls {
		// if module is local, calculate hash recursively
		// otherwise, module name and version is enough
		var h *ModuleInfo
		if strings.HasPrefix(mc.Source, ".") {
			h, err = calcModuleHash(k, filepath.Join(dir, mc.Source))
			if err != nil {
				return nil, err
			}
		} else {
			h = &ModuleInfo{
				Name: mc.Source,
				Files: []*FileInfo{
					{
						Name: "::remote::",
						Hash: mc.Source + "@" + mc.Version,
					},
				},
			}
		}
		deps = append(deps, h)
	}

	sort.Slice(fileHashes, func(i, j int) bool {
		return fileHashes[i].Name < fileHashes[j].Name
	})
	sort.Slice(deps, func(i, j int) bool {
		return deps[i].Name < deps[j].Name
	})

	// list modules
	return &ModuleInfo{
		Name:  name,
		Files: fileHashes,
		Deps:  deps,
	}, err
}

// calculate file hash with SHA-1 algorithm
func calcFileHash(fname string) (string, error) {
	f, err := os.Open(fname)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := hasher()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func hasher() hash.Hash {
	return sha1.New()
}
