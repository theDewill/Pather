package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type Store map[string]map[string]string

func main() {
	saveMode := flag.Bool("s", false, "save mode")
	runMode := flag.Bool("r", false, "run saved path")
	deleteMode := flag.Bool("d", false, "delete a saved nickname from a block")
	block := flag.String("b", "", "block name")
	nickname := flag.String("n", "", "nickname inside the block")

	flag.Usage = func() {
		fmt.Println(`pather - save, list, and run tool paths

Usage:
  Save:
    pather -s -b <block> -n <nickname> /path/to/tool

  List all blocks:
    pather -b

  List all nicknames and paths in a block:
    pather -b <block>

  Run a saved tool:
    pather -r -b <block> -n <nickname> [args...]

  Delete a saved nickname:
    pather -d -b <block> -n <nickname>

Examples:
  pather -s -b xcode -n stable /Applications/Xcode.app
  pather -s -b tools -n go /opt/homebrew/bin/go
  pather -b
  pather -b xcode
  pather -r -b tools -n go version
  pather -d -b tools -n go
`)
	}

	flag.Parse()

	store, storePath, err := loadStore()
	if err != nil {
		exitErr(err)
	}

	switch {
	case *saveMode:
		if err := handleSave(store, storePath, *block, *nickname, flag.Args()); err != nil {
			exitErr(err)
		}
	case *runMode:
		if err := handleRun(store, *block, *nickname, flag.Args()); err != nil {
			exitErr(err)
		}
	case *deleteMode:
		if err := handleDelete(store, storePath, *block, *nickname); err != nil {
			exitErr(err)
		}
	default:
		if err := handleList(store, os.Args[1:]); err != nil {
			exitErr(err)
		}
	}
}

func handleSave(store Store, storePath, block, nickname string, args []string) error {
	if strings.TrimSpace(block) == "" {
		return errors.New("missing block name, use -b <block>")
	}
	if strings.TrimSpace(nickname) == "" {
		return errors.New("missing nickname, use -n <nickname>")
	}
	if len(args) != 1 {
		return errors.New("save mode requires exactly one path argument")
	}

	rawPath := args[0]
	expanded, err := expandPath(rawPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	if _, err := os.Stat(expanded); err != nil {
		return fmt.Errorf("path does not exist: %s", expanded)
	}

	if _, ok := store[block]; !ok {
		store[block] = map[string]string{}
	}

	store[block][nickname] = expanded

	if err := saveStore(storePath, store); err != nil {
		return err
	}

	fmt.Printf("Saved [%s/%s] -> %s\n", block, nickname, expanded)
	return nil
}

func handleRun(store Store, block, nickname string, args []string) error {
	if strings.TrimSpace(block) == "" {
		return errors.New("missing block name, use -b <block>")
	}
	if strings.TrimSpace(nickname) == "" {
		return errors.New("missing nickname, use -n <nickname>")
	}

	blockMap, ok := store[block]
	if !ok {
		return fmt.Errorf("block not found: %s", block)
	}

	target, ok := blockMap[nickname]
	if !ok {
		return fmt.Errorf("nickname not found: %s in block %s", nickname, block)
	}

	cmd := exec.Command(target, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func handleDelete(store Store, storePath, block, nickname string) error {
	if strings.TrimSpace(block) == "" {
		return errors.New("missing block name, use -b <block>")
	}
	if strings.TrimSpace(nickname) == "" {
		return errors.New("missing nickname, use -n <nickname>")
	}

	blockMap, ok := store[block]
	if !ok {
		return fmt.Errorf("block not found: %s", block)
	}

	if _, ok := blockMap[nickname]; !ok {
		return fmt.Errorf("nickname not found: %s in block %s", nickname, block)
	}

	delete(blockMap, nickname)
	if len(blockMap) == 0 {
		delete(store, block)
	}

	if err := saveStore(storePath, store); err != nil {
		return err
	}

	fmt.Printf("Deleted [%s/%s]\n", block, nickname)
	return nil
}

func handleList(store Store, rawArgs []string) error {
	// Expected behaviors:
	// pather -b        -> list all blocks
	// pather -b xcode  -> list all items in block xcode
	//
	// Because Go's flag package treats -b as requiring a value,
	// we also support:
	//   pather -b=
	//   pather -b xcode
	//
	// To support exactly "pather -b", we inspect raw args.

	if len(rawArgs) == 1 && rawArgs[0] == "-b" {
		listBlocks(store)
		return nil
	}

	// If user provided -b <block>, the flag package already parsed it.
	// Recover parsed value from standard flag set.
	parsedBlock := flag.Lookup("b").Value.String()

	if parsedBlock == "" {
		flag.Usage()
		return nil
	}

	listBlockEntries(store, parsedBlock)
	return nil
}

func listBlocks(store Store) {
	if len(store) == 0 {
		fmt.Println("No blocks saved.")
		return
	}

	blocks := make([]string, 0, len(store))
	for block := range store {
		blocks = append(blocks, block)
	}
	sort.Strings(blocks)

	fmt.Println("Blocks:")
	for _, block := range blocks {
		fmt.Printf("  - %s (%d)\n", block, len(store[block]))
	}
}

func listBlockEntries(store Store, block string) {
	blockMap, ok := store[block]
	if !ok {
		fmt.Printf("Block not found: %s\n", block)
		return
	}

	if len(blockMap) == 0 {
		fmt.Printf("Block '%s' is empty.\n", block)
		return
	}

	keys := make([]string, 0, len(blockMap))
	for k := range blockMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fmt.Printf("Block: %s\n", block)
	for _, k := range keys {
		fmt.Printf("  %s -> %s\n", k, blockMap[k])
	}
}

func loadStore() (Store, string, error) {
	storePath, err := getStorePath()
	if err != nil {
		return nil, "", err
	}

	if err := os.MkdirAll(filepath.Dir(storePath), 0o755); err != nil {
		return nil, "", fmt.Errorf("failed to create config dir: %w", err)
	}

	store := Store{}

	data, err := os.ReadFile(storePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return store, storePath, nil
		}
		return nil, "", fmt.Errorf("failed to read store file: %w", err)
	}

	if len(data) == 0 {
		return store, storePath, nil
	}

	if err := json.Unmarshal(data, &store); err != nil {
		return nil, "", fmt.Errorf("failed to parse store file: %w", err)
	}

	return store, storePath, nil
}

func saveStore(storePath string, store Store) error {
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode store: %w", err)
	}

	if err := os.WriteFile(storePath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write store file: %w", err)
	}

	return nil
}

func getStorePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user config dir: %w", err)
	}

	return filepath.Join(configDir, "pather", "store.json"), nil
}

func expandPath(p string) (string, error) {
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if p == "~" {
			p = home
		} else if strings.HasPrefix(p, "~/") {
			p = filepath.Join(home, p[2:])
		}
	}

	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}

	return abs, nil
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, "Error:", err)
	os.Exit(1)
}
