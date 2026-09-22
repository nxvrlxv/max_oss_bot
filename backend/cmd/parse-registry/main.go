// Command parse-registry prints a registry CSV as a JSON array.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"oss-max/internal/registry"
)

func main() {
	output := flag.String("out", "", "save UTF-8 JSON to a new file instead of stdout")
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "Usage: parse-registry [-out result.json] registry.csv")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(flag.Arg(0), *output); err != nil {
		fmt.Fprintln(os.Stderr, "parse-registry:", err)
		os.Exit(1)
	}
}

func run(input, output string) error {
	f, err := os.Open(input)
	if err != nil {
		return err
	}
	defer f.Close()
	rows, err := registry.ParseCSV(f)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if output == "" {
		_, err = os.Stdout.Write(data)
		return err
	}
	// Exclusive creation protects both the input and existing result files.
	dest, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0666)
	if err != nil {
		return err
	}
	_, writeErr := dest.Write(data)
	closeErr := dest.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	fmt.Fprintf(os.Stderr, "Parsed %d ownership records. Saved to %s\n", len(rows), output)
	return nil
}
