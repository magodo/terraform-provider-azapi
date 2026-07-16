package main

import (
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"

	"github.com/Azure/bicep-types/src/bicep-types-go/types"
	"github.com/Azure/terraform-provider-azapi/internal/azure"
)

type bicepTypeLoader struct {
	files map[string][]types.Type
}

type bicepTypeKey struct {
	file string
	idx  int
}

type bicepType struct {
	t   types.Type
	key bicepTypeKey
}

func newBicepTypeLoader() bicepTypeLoader {
	return bicepTypeLoader{files: map[string][]types.Type{}}
}

// resolve follows a type reference relative to the given file.
func (l *bicepTypeLoader) resolve(file string, ref types.ITypeReference) (bicepType, error) {
	var index int
	switch r := ref.(type) {
	case types.TypeReference:
		index = r.Ref
	case types.CrossFileTypeReference:
		if r.RelativePath != "" {
			file = filepath.Join(filepath.Dir(file), r.RelativePath)
		}
		index = r.Ref
	default:
		return bicepType{}, fmt.Errorf("unsupported type reference %T", ref)
	}

	t, err := l.typeAt(file, index)
	if err != nil {
		return bicepType{}, err
	}
	return bicepType{
		t:   t,
		key: bicepTypeKey{file: file, idx: index},
	}, nil
}

func (l *bicepTypeLoader) typeAt(file string, idx int) (types.Type, error) {
	ts, ok := l.files[file]
	if !ok {
		b, err := azure.StaticFiles.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read types file %q: %w", file, err)
		}
		var raw []json.RawMessage
		if err := json.Unmarshal(b, &raw); err != nil {
			return nil, fmt.Errorf("failed to unmarshal types array %q: %w", file, err)
		}
		ts = make([]types.Type, len(raw))
		for i, r := range raw {
			// Individual types that fail to decode are left nil; only relevant
			// if actually referenced.
			t, err := types.UnmarshalType(r)
			if err != nil {
				log.Printf("[WARN] failed to unmarshal %d-th type in %q: %v. %v", i, file, err, string(r))
				continue
			}
			ts[i] = t
		}
		l.files[file] = ts
	}
	if idx < 0 || idx >= len(ts) {
		return nil, fmt.Errorf("type index %d out of range for file %q", idx, file)
	}
	if ts[idx] == nil {
		return nil, fmt.Errorf("type index %d in file %q failed to parse", idx, file)
	}
	return ts[idx], nil
}
