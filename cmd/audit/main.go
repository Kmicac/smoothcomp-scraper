package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/kmicac/smoothcomp-scraper/internal/adapters/smoothcomp/audit"
)

func main() {
	var (
		datasetPath = flag.String("dataset", "testdata/smoothcomp/audit/dataset.json", "path to audit dataset")
		format      = flag.String("format", "markdown", "output format: markdown or json")
	)
	flag.Parse()

	dataset, err := audit.LoadDataset(*datasetPath)
	if err != nil {
		exitf("load dataset: %v", err)
	}

	report, err := audit.NewDefaultRunner().Run(context.Background(), dataset)
	if err != nil {
		exitf("run audit: %v", err)
	}

	switch *format {
	case "markdown":
		fmt.Print(report.Markdown())
	case "json":
		body, err := report.JSON()
		if err != nil {
			exitf("encode report: %v", err)
		}
		fmt.Println(string(body))
	default:
		exitf("unsupported format: %s", *format)
	}
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
