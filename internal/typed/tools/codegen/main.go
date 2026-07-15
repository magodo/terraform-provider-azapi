package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
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
                     [--remove-attr PATH]... [--add-attr PATH]...
  codegen datasource ...   (not implemented)

Path filters:
  --remove-attr and --add-attr take a dot-separated API path (camelCase).
  Every array/map element boundary must be represented by a literal "*"
  segment (matching how attribute paths are constructed internally). Both
  flags may be repeated; their command-line order is significant and a later
  rule overrides earlier ones for overlapping paths.

Example (rescue only the .id of a pruned subtree under an array element):
  codegen resource --api-type "Microsoft.Network/virtualNetworks@2025-01-01" --tf-type "azapi_virtual_network" \
    --remove-attr properties.subnets.*.properties.networkSecurityGroup \
    --add-attr    properties.subnets.*.properties.networkSecurityGroup.id

Reversing the order of the two rules above prunes the whole subtree
(--add-attr becomes a no-op, then --remove-attr wins).
`)
}

// ruleFlag is a repeatable flag.Value that appends attrRule entries to a
// shared ordered list, so the CLI order of --remove-attr / --add-attr is
// preserved.
type ruleFlag struct {
	list    *[]attrRule
	include bool
}

func (f ruleFlag) String() string {
	if f.list == nil {
		return ""
	}
	parts := make([]string, 0, len(*f.list))
	for _, r := range *f.list {
		if r.include == f.include {
			parts = append(parts, strings.Join(r.path, "."))
		}
	}
	return strings.Join(parts, ",")
}

func (f ruleFlag) Set(v string) error {
	v = strings.TrimSpace(v)
	if v == "" {
		return fmt.Errorf("empty path")
	}
	*f.list = append(*f.list, attrRule{path: strings.Split(v, "."), include: f.include})
	return nil
}

func runResource(args []string) error {
	fs := flag.NewFlagSet("resource", flag.ExitOnError)
	apiType := fs.String("api-type", "", `the Azure resource type in the form "<ResourceType>@<ApiVersion>", e.g. "Microsoft.Network/virtualNetworks@2025-01-01"`)
	tfType := fs.String("tf-type", "", `the terraform resource type, e.g. "azapi_virtual_network"`)
	output := fs.String("output", "", "output directory (defaults to cwd)")
	stdout := fs.Bool("stdout", false, "write the generated code to stdout instead of a file")
	var rules []attrRule
	fs.Var(ruleFlag{list: &rules, include: false}, "remove-attr", "dot-separated API path to prune from the schema (repeatable; order-sensitive with --add-attr)")
	fs.Var(ruleFlag{list: &rules, include: true}, "add-attr", "dot-separated API path to re-include (repeatable; order-sensitive with --remove-attr)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *apiType == "" {
		return fmt.Errorf("--api-type is required")
	}
	if *tfType == "" {
		return fmt.Errorf("--tf-type is required")
	}

	g, err := NewResourceGenerator(*apiType, *tfType, rules)
	if err != nil {
		return fmt.Errorf("failed to new resource generator: %v", err)
	}

	src, err := g.Generate()
	if err != nil {
		return err
	}

	if *stdout {
		_, err = os.Stdout.Write(src)
		return err
	}

	dir := *output
	if dir == "" {
		cwd, _ := os.Getwd()
		dir = cwd
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
