// Package model resolves models from the registry (configs/models.yaml) and
// downloads their GGUF files with resumable transfers and SHA-256 verification.
//
// It intentionally holds no model URLs of its own: the download location comes
// entirely from config.ModelSource, so switching hosts or mirrors is a config
// change. The repository never contains the model itself — it is fetched at
// install time.
package model

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rezonmain/loca-agent/internal/config"
	"github.com/rezonmain/loca-agent/internal/errs"
)

// Resolve returns the requested model, or the registry default when id is
// empty. On an unknown id it returns an actionable UserError listing the
// available ids.
func Resolve(reg config.ModelRegistry, id string) (config.Model, error) {
	if id == "" {
		id = reg.Default
	}
	if m, ok := reg.Find(id); ok {
		return m, nil
	}
	return config.Model{}, errs.New("model_unknown",
		fmt.Sprintf("model %q is not in the registry", id),
		fmt.Sprintf("Choose one of: %s", strings.Join(availableIDs(reg), ", ")))
}

// FileURL builds the download URL for one file of a model by expanding the
// source template with the {base}, {repo}, and {file} placeholders.
func FileURL(src config.ModelSource, m config.Model, f config.ModelFile) string {
	r := strings.NewReplacer(
		"{base}", strings.TrimRight(src.BaseURL, "/"),
		"{repo}", m.Repo,
		"{file}", f.Name,
	)
	return r.Replace(src.FileURLTemplate)
}

func availableIDs(reg config.ModelRegistry) []string {
	ids := make([]string, 0, len(reg.Models))
	for _, m := range reg.Models {
		ids = append(ids, m.ID)
	}
	sort.Strings(ids)
	return ids
}
