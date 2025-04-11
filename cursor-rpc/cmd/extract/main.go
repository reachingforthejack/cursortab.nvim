package main

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	//go:embed preprocess.js
	PreProcess string

	//go:embed postprocess.js
	PostProcess string
)

func bailIf(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

const extensionHostProcFilePath = "Contents/Resources/app/out/vs/workbench/api/node/extensionHostProcess.js"

func main() {
	if runtime.GOOS != "darwin" {
		bailIf(fmt.Errorf("only supports getting RPC schema from macos binary"))
	}
	args := os.Args
	if len(args) > 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s [cursor binary location]\n", args[0])
		os.Exit(1)
	}

	prettierBin, err := exec.LookPath("prettier")
	bailIf(err)

	nodeBin, err := exec.LookPath("node")
	bailIf(err)

	cursorPath := "/Applications/Cursor.app"
	if len(args) > 1 {
		cursorPath = args[1]
	}

	cursorLocation, err := filepath.Abs(cursorPath)
	bailIf(err)

	ehpPath := filepath.Join(cursorLocation, extensionHostProcFilePath)

	info, err := os.Stat(ehpPath)
	bailIf(err)

	if info.IsDir() {
		bailIf(fmt.Errorf("Expected %s to be file, is dir", ehpPath))
	}

	originalFile, err := os.Open(ehpPath)
	bailIf(err)

	generateSchemaScript, err := os.CreateTemp(os.TempDir(), "cursor-rpc-*.js")
	bailIf(err)

	generateSchemaScriptName := generateSchemaScript.Name()

	_, err = io.Copy(generateSchemaScript, originalFile)
	bailIf(err)

	bailIf(originalFile.Close())
	bailIf(generateSchemaScript.Close())

	// Format the file so we can read it line by line
	prettierCmd := exec.Command(prettierBin, "--write", generateSchemaScriptName)
	out, err := prettierCmd.CombinedOutput()
	if err != nil {
		fmt.Printf(string(out))
		bailIf(err)
	}

	// Re-open the file, read line-by-line to find the critical parts
	content, err := os.ReadFile(generateSchemaScriptName)
	bailIf(err)

	schemaGeneratorFile, err := os.CreateTemp(os.TempDir(), "cursor-rpc-decorated-*.js")
	bailIf(err)

	var foundDefine bool
	for _, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)

		if !foundDefine && strings.HasPrefix(trimmed, "define(") {
			foundDefine = true
			_, err = schemaGeneratorFile.WriteString(PreProcess)
			bailIf(err)
		}

		if trimmed == "}).call(this);" {
			_, err = schemaGeneratorFile.WriteString(PostProcess)
			bailIf(err)
		}

		_, err = schemaGeneratorFile.WriteString(line + "\n")
		bailIf(err)
	}

	schemaGeneratorName := schemaGeneratorFile.Name()
	bailIf(schemaGeneratorFile.Close())

	wd, err := os.Getwd()
	bailIf(err)

	nodeCmd := exec.Command(nodeBin, schemaGeneratorName, filepath.Join(wd, "cursor"))
	out, err = nodeCmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, string(out))
		bailIf(err)
	}
}
