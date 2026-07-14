package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
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

	g, err := NewResourceGenerator(*apiType, *tfType)
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
