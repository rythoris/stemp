// Stemp - the simple templating program.
//
// Copyright (C) 2024 Ryan Thoris
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	htemplate "html/template"
	template "text/template"

	"github.com/alexflint/go-arg"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

// TODO: automated testing

const VERSION = "0.1.0"

var defaultFuncs = template.FuncMap{
	"inc": func(a int64) int64 { return a + 1 },
	"dec": func(a int64) int64 { return a + 1 },

	"add": func(a, b float64) float64 { return a + b },
	"sub": func(a, b float64) float64 { return a - b },
	"div": func(a, b float64) float64 { return a / b },
	"mul": func(a, b float64) float64 { return a * b },
	"mod": func(a, b int64) int64 { return a % b },

	"sin": math.Sin,
	"cos": math.Cos,
	"tan": math.Tan,

	"abs":   math.Abs,
	"floor": math.Floor,
	"ceil":  math.Ceil,

	"join":        strings.Join,
	"trim":        strings.TrimSpace,
	"trim_prefix": strings.TrimPrefix,
	"trim_suffix": strings.TrimSuffix,
	"has_prefix":  strings.HasPrefix,
	"has_suffix":  strings.HasSuffix,
	"upper":       strings.ToUpper,
	"lower":       strings.ToLower,
	"title":       strings.ToTitle,
}

type Cli struct {
	TemplateFile string `arg:"positional" placeholder:"TEMPLATE" help:"template file"`
	VarsFile     string `arg:"positional" placeholder:"VARS_FILE" help:"variables file"`

	VarsFormat   string `arg:"-f,--vars-format" help:"implicitly specify the vars-file format (supported formats: json, yaml, toml)"`
	OutputFile   string `arg:"-o,--output" help:"specify output file, by default it will print the result to stdout."`
	HtmlTemplate bool   `arg:"-H, --html" help:"html mode. uses html/template module instead of text/template."`

	Version bool `arg:"-v,--version" help:"display program version and exit"`
}

func (Cli) Description() string {
	return "Stemp - the simple templating program.\n"
}

// TODO: include examples in the Epilogue
// func (Cli) Epilogue() string {
// 	sb := &strings.Builder{}
// 	return sb.String()
// }

func main() {
	var err error
	var cli Cli
	parser := arg.MustParse(&cli)

	if cli.Version {
		fmt.Println("stemp " + VERSION)
		fmt.Println()
		fmt.Println("Source Code: https://github.com/rythoris/stemp")
		fmt.Println("Bug Tracker: https://github.com/rythoris/stemp/issues")

		os.Exit(1)
	}
	_ = parser

	if len(cli.TemplateFile) == 0 {
		fatalf("ERROR: TEMPLATE file is not provided.\n")
	} else if len(cli.VarsFile) == 0 {
		fatalf("ERROR: VARS_FILE is not provided.\n")
	} else if cli.TemplateFile == "-" && cli.VarsFile == "-" {
		fatalf("ERROR: only one of the VARS_FILE or TEMPLATE can use stdin.\n")
	}

	var unmarshalFunc func([]byte, any) error
	if cli.VarsFile == "-" && len(cli.VarsFormat) == 0 {
		fatalf("ERROR: you must provide the '--vars-format' when you're using the stdin as VARS_FILE.\n")
	}

	format := cli.VarsFormat
	if cli.VarsFile != "-" {
		format = strings.TrimLeft(filepath.Ext(cli.VarsFile), ".")
	}

	switch format {
	case "json":
		unmarshalFunc = json.Unmarshal
	case "yml":
		fallthrough
	case "yaml":
		unmarshalFunc = yaml.Unmarshal
	case "toml":
		unmarshalFunc = toml.Unmarshal
	default:
		if len(cli.VarsFormat) == 0 {
			fatalf("ERROR: invalid '--vars-format' value: %s\n", cli.VarsFormat)
		} else {
			fatalf("ERROR: could not detect file-type using the file extention: %s\n", cli.VarsFile)
		}
	}

	var outputFile *os.File = os.Stdout
	if len(cli.OutputFile) > 0 {
		outputFile, err = os.OpenFile(cli.OutputFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0744)
		if err != nil {
			fatalf("ERROR: could not open output file: %s: %s\n", cli.OutputFile, err.Error())
		}
	}

	templateFileContent, err := readFileContent(cli.TemplateFile)
	if err != nil {
		fatalf("ERROR: could not read template file: %s\n", err.Error())
	}

	varsFileContent, err := readFileContent(cli.VarsFile)
	if err != nil {
		fatalf("ERROR: could not read vars file: %s\n", err.Error())
	}

	var Vars any
	err = unmarshalFunc(varsFileContent, &Vars)
	if err != nil {
		fatalf("ERROR: unmarshall error: %s\n", err.Error())
	}

	// I know, I know... but It fucking works!
	type DummyTemplateInterface interface {
		Execute(io.Writer, any) error
	}

	t, err := func() (DummyTemplateInterface, error) {
		if cli.HtmlTemplate {
			return htemplate.New("template").Funcs(defaultFuncs).Parse(string(templateFileContent))
		} else {
			return template.New("template").Funcs(defaultFuncs).Parse(string(templateFileContent))
		}
	}()
	if err != nil {
		fatalf("ERROR: could not parse template file: %s\n", err.Error())
	}
	if err := t.Execute(outputFile, Vars); err != nil {
		fatalf("ERROR: could execute template: %s\n", err.Error())
	}
}

func fatalf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format, a...)
	os.Exit(1)
}

func readFileContent(filePath string) ([]byte, error) {
	var (
		content []byte
		err     error
	)

	if filePath == "-" {
		content, err = io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("stdin: %w", err)
		}
	} else {
		content, err = os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filePath, err)
		}
	}

	return content, nil
}
