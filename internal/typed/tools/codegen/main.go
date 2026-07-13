package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Azure/bicep-types/src/bicep-types-go/types"
	"github.com/Azure/terraform-provider-azapi/internal/azure"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "resource":
		if err := runResource(os.Args[2:]); err != nil {
			log.Fatalf("error: %v", err)
		}
	case "datasource":
		log.Fatalf("datasource generation is not implemented yet")
	case "-h", "--help", "help":
		usage()
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `codegen generates typed azapi resources/datasources from the bicep types.

Usage:
  codegen resource   --api-type "<ResourceType>@<ApiVersion>" --tf-type "azapi_xxx" [--output DIR]
  codegen datasource ...   (not implemented)

Example:
  codegen resource --api-type "Microsoft.Network/virtualNetworks@2025-01-01" --tf-type "azapi_virtual_network"
`)
}

func runResource(args []string) error {
	fs := flag.NewFlagSet("resource", flag.ExitOnError)
	apiType := fs.String("api-type", "", `the Azure resource type in the form "<ResourceType>@<ApiVersion>", e.g. "Microsoft.Network/virtualNetworks@2025-01-01"`)
	tfType := fs.String("tf-type", "", `the terraform resource type, e.g. "azapi_virtual_network"`)
	output := fs.String("output", "", "output directory (defaults to internal/typed/services/<service>)")
	stdout := fs.Bool("stdout", false, "write the generated code to stdout instead of a file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *apiType == "" {
		return fmt.Errorf("--api-type is required")
	}
	if *tfType == "" {
		return fmt.Errorf("--tf-type is required")
	}

	resourceType, apiVersion, ok := strings.Cut(*apiType, "@")
	if !ok {
		return fmt.Errorf("invalid --api-type %q, expected format <ResourceType>@<ApiVersion>", *apiType)
	}

	loader, err := newTypeLoader()
	if err != nil {
		return err
	}

	res, err := loader.GetResource(resourceType, apiVersion)
	if err != nil {
		return err
	}

	g := &generator{
		loader:        loader,
		apiType:       *apiType,
		resourceType:  resourceType,
		apiVersion:    apiVersion,
		tfType:        *tfType,
		nameOverrides: map[string]string{},
		imports:       newImportSet(),
	}

	src, err := g.Generate(res)
	if err != nil {
		return err
	}

	if *stdout {
		_, err = os.Stdout.Write(src)
		return err
	}

	dir := *output
	if dir == "" {
		dir = filepath.Join("internal", "typed", "services", g.serviceName())
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory %q: %w", dir, err)
	}
	outPath := filepath.Join(dir, g.fileBaseName()+"_resource_gen.go")
	if err := os.WriteFile(outPath, src, 0o644); err != nil {
		return fmt.Errorf("failed to write %q: %w", outPath, err)
	}
	log.Printf("generated %s", outPath)
	return nil
}

// -----------------------------------------------------------------------------
// Bicep types loading
// -----------------------------------------------------------------------------

// indexRaw is a minimal representation of generated/index.json. We intentionally
// do NOT use bicep-types-go/index here because the on-disk index encodes
// resourceFunctions in an array-based shape that its TypeIndex.UnmarshalJSON
// rejects. We only need the resource -> file reference mapping.
type indexRaw struct {
	Resources map[string]indexRef `json:"resources"`
}

type indexRef struct {
	Ref string `json:"$ref"`
}

// typeLoader resolves bicep type references, loading (and caching) the referenced
// type files on demand.
type typeLoader struct {
	index indexRaw
	files map[string][]types.Type
}

func newTypeLoader() (*typeLoader, error) {
	b, err := azure.StaticFiles.ReadFile("generated/index.json")
	if err != nil {
		return nil, fmt.Errorf("failed to load schema index: %w", err)
	}
	var idx indexRaw
	if err := json.Unmarshal(b, &idx); err != nil {
		return nil, fmt.Errorf("failed to unmarshal schema index: %w", err)
	}
	return &typeLoader{
		index: idx,
		files: map[string][]types.Type{},
	}, nil
}

// resolvedResource bundles a resource type together with the file it lives in so
// that same-file references can be resolved.
type resolvedResource struct {
	resource *types.ResourceType
	file     string
}

func (l *typeLoader) GetResource(resourceType, apiVersion string) (*resolvedResource, error) {
	// index keys are stored as "<ResourceType>@<ApiVersion>". Match case-insensitively.
	key := resourceType + "@" + apiVersion
	ref, ok := l.index.Resources[key]
	if !ok {
		for k, v := range l.index.Resources {
			if strings.EqualFold(k, key) {
				ref, ok = v, true
				break
			}
		}
	}
	if !ok {
		return nil, fmt.Errorf("resource type %q (api-version %q) not found in the bicep index", resourceType, apiVersion)
	}

	relPath, refIdx, err := parseRef(ref.Ref)
	if err != nil {
		return nil, err
	}
	t, err := l.typeAt(relPath, refIdx)
	if err != nil {
		return nil, err
	}
	rt, ok := t.(*types.ResourceType)
	if !ok {
		return nil, fmt.Errorf("index entry for %q is not a ResourceType (got %T)", key, t)
	}
	return &resolvedResource{resource: rt, file: relPath}, nil
}

// Resolve follows a type reference relative to the given file, returning the
// referenced concrete type, the file it lives in (for further resolution), and
// the numeric index within that file (for cycle detection).
func (l *typeLoader) Resolve(file string, ref types.ITypeReference) (types.Type, string, int, error) {
	switch r := ref.(type) {
	case types.TypeReference:
		t, err := l.typeAt(file, r.Ref)
		return t, file, r.Ref, err
	case types.CrossFileTypeReference:
		relPath := normalizeRelPath(file, r.RelativePath)
		t, err := l.typeAt(relPath, r.Ref)
		return t, relPath, r.Ref, err
	default:
		return nil, "", 0, fmt.Errorf("unsupported type reference %T", ref)
	}
}

func (l *typeLoader) typeAt(relPath string, idx int) (types.Type, error) {
	ts, err := l.load(relPath)
	if err != nil {
		return nil, err
	}
	if idx < 0 || idx >= len(ts) {
		return nil, fmt.Errorf("type index %d out of range for file %q", idx, relPath)
	}
	if ts[idx] == nil {
		return nil, fmt.Errorf("type index %d in file %q failed to parse", idx, relPath)
	}
	return ts[idx], nil
}

func (l *typeLoader) load(relPath string) ([]types.Type, error) {
	if ts, ok := l.files[relPath]; ok {
		return ts, nil
	}
	b, err := azure.StaticFiles.ReadFile(filepath.Join("generated", relPath))
	if err != nil {
		return nil, fmt.Errorf("failed to read types file %q: %w", relPath, err)
	}
	ts, err := loadBicepTypes(b)
	if err != nil {
		return nil, err
	}
	l.files[relPath] = ts
	return ts, nil
}

// loadBicepTypes decodes a Bicep types JSON document from raw bytes. Individual
// types that fail to decode (e.g. malformed ResourceFunctionType entries that we
// never reference) are left nil rather than failing the whole document.
func loadBicepTypes(data []byte) ([]types.Type, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to unmarshal types array: %w", err)
	}

	result := make([]types.Type, len(raw))
	for i, r := range raw {
		typ, err := types.UnmarshalType(r)
		if err != nil {
			// Leave nil; only relevant if actually referenced.
			continue
		}
		result[i] = typ
	}
	return result, nil
}

func parseRef(ref string) (relPath string, idx int, err error) {
	parts := strings.SplitN(ref, "#/", 2)
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid type reference %q", ref)
	}
	idx, err = strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, fmt.Errorf("invalid type reference index in %q: %w", ref, err)
	}
	return parts[0], idx, nil
}

// normalizeRelPath resolves a cross-file relative path against the file that
// references it.
func normalizeRelPath(fromFile, relativePath string) string {
	if relativePath == "" {
		return fromFile
	}
	return filepath.ToSlash(filepath.Join(filepath.Dir(fromFile), relativePath))
}
