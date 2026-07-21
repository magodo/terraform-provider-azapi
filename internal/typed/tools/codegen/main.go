// Command codegen generates static typed AzAPI resources from the Azure bicep
// types.
//
// Given a single API type at a single API version, it emits a vanilla
// "<name>_resource_gen.go" whose schema mirrors the API model. The generated
// code is meant to be committed as-is and never hand-edited; customization of
// schema and lifecycle behavior belongs in a sibling, hand-written resource
// file.
//
// See internal/typed/README.md for the wider workflow.
//
// Usage:
//
//	codegen resource --api-type <ResourceType>@<ApiVersion> --tf-type <azapi_xxx> [options]
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/cli"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	ui := &cli.BasicUi{Reader: stdin, Writer: stdout, ErrorWriter: stderr}
	app := cli.NewCLI("codegen", "")
	app.Args = args
	app.Commands = map[string]cli.CommandFactory{
		"resource": func() (cli.Command, error) {
			return &resourceCommand{ui: ui, stdout: stdout}, nil
		},
		"datasource": func() (cli.Command, error) {
			return &unimplementedCommand{ui: ui, name: "datasource"}, nil
		},
	}
	app.HelpWriter = stdout
	app.ErrorWriter = stderr

	exitCode, err := app.Run()
	if err != nil {
		ui.Error(err.Error())
		return 1
	}
	return exitCode
}

type resourceCommand struct {
	ui     cli.Ui
	stdout io.Writer
}

func (c *resourceCommand) Synopsis() string {
	return "generate a typed resource from the bicep types"
}

func (c *resourceCommand) Help() string {
	return strings.TrimSpace(`
Usage: codegen resource [options]

  Generates a typed AzAPI resource from the bicep types.

Options:
  --api-type TYPE       Azure resource type as <ResourceType>@<ApiVersion>.
  --tf-type TYPE        Terraform resource type, for example azapi_virtual_network.
  --output DIR          Output directory. Defaults to the current directory.
  --stdout              Write generated code to stdout instead of a file.
  --remove-attr PATH    Prune a dot-separated API path. May be repeated.
  --add-attr PATH       Re-include a dot-separated API path. May be repeated.

Attribute rules are order-sensitive. Every array or map element boundary must
be represented by a literal "*" path segment. Later overlapping rules win.
`)
}

func (c *resourceCommand) Run(args []string) int {
	var options struct {
		apiType string
		tfType  string
		output  string
		stdout  bool
		rules   []attrRule
	}

	flags := flag.NewFlagSet("resource", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&options.apiType, "api-type", "", "Azure resource type")
	flags.StringVar(&options.tfType, "tf-type", "", "Terraform resource type")
	flags.StringVar(&options.output, "output", "", "output directory")
	flags.BoolVar(&options.stdout, "stdout", false, "write generated code to stdout")
	flags.Var(ruleFlag{list: &options.rules, include: false}, "remove-attr", "API path to prune")
	flags.Var(ruleFlag{list: &options.rules, include: true}, "add-attr", "API path to re-include")
	if err := flags.Parse(args); err != nil {
		c.ui.Error(fmt.Sprintf("failed to parse flags: %v", err))
		return 1
	}
	if flags.NArg() != 0 {
		c.ui.Error(fmt.Sprintf("unexpected arguments: %s", strings.Join(flags.Args(), " ")))
		return 1
	}
	if options.apiType == "" {
		c.ui.Error("--api-type is required")
		return 1
	}
	if options.tfType == "" {
		c.ui.Error("--tf-type is required")
		return 1
	}

	generator, err := newResourceGenerator(resourceGeneratorOptions{
		apiType: options.apiType,
		tfType:  options.tfType,
		rules:   options.rules,
	})
	if err != nil {
		c.ui.Error(fmt.Sprintf("failed to create resource generator: %v", err))
		return 1
	}
	source, err := generator.Generate()
	if err != nil {
		c.ui.Error(err.Error())
		return 1
	}
	if options.stdout {
		if _, err := c.stdout.Write(source); err != nil {
			c.ui.Error(fmt.Sprintf("failed to write generated code: %v", err))
			return 1
		}
		return 0
	}

	output := options.output
	if output == "" {
		output, err = os.Getwd()
		if err != nil {
			c.ui.Error(fmt.Sprintf("failed to determine current directory: %v", err))
			return 1
		}
	}
	if err := os.MkdirAll(output, 0o755); err != nil {
		c.ui.Error(fmt.Sprintf("failed to create output directory %q: %v", output, err))
		return 1
	}
	outputPath := filepath.Join(output, generator.fileBaseName()+"_resource_gen.go")
	if err := os.WriteFile(outputPath, source, 0o644); err != nil {
		c.ui.Error(fmt.Sprintf("failed to write %q: %v", outputPath, err))
		return 1
	}
	c.ui.Output(fmt.Sprintf("generated %s", outputPath))
	return 0
}

type unimplementedCommand struct {
	ui   cli.Ui
	name string
}

func (c *unimplementedCommand) Synopsis() string { return "not implemented" }
func (c *unimplementedCommand) Help() string     { return "This command is not implemented." }
func (c *unimplementedCommand) Run([]string) int {
	c.ui.Error(c.name + " generation is not implemented yet")
	return 1
}

// ruleFlag preserves the command-line order of repeatable include/exclude rules.
type ruleFlag struct {
	list    *[]attrRule
	include bool
}

func (f ruleFlag) String() string {
	if f.list == nil {
		return ""
	}
	var paths []string
	for _, rule := range *f.list {
		paths = append(paths, strings.Join(rule.path, "."))
	}
	return strings.Join(paths, ",")
}

func (f ruleFlag) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("empty path")
	}
	*f.list = append(*f.list, attrRule{path: strings.Split(value, "."), include: f.include})
	return nil
}
