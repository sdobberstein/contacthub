// Command seed creates a test user and app password in the dev database,
// then writes the generated token directly into davlint.yaml so no manual
// copy-paste is needed.
//
// Usage:
//
//	go run ./cmd/seed [--db ./dev.db] [--username alice] [--name "Alice"] [--label "davlint"] [--davlint-config ./davlint.yaml]
package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"

	"github.com/sdobberstein/contacthub/internal/auth/local"
	"github.com/sdobberstein/contacthub/internal/migrations"
	"github.com/sdobberstein/contacthub/internal/store"
	"github.com/sdobberstein/contacthub/internal/store/sqlite"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		dbPath       string
		username     string
		name         string
		password     string
		label        string
		davlintCfg   string
	)
	flag.StringVar(&dbPath, "db", "./dev.db", "path to SQLite database")
	flag.StringVar(&username, "username", "alice", "username to create (or reuse)")
	flag.StringVar(&name, "name", "", "display name (defaults to username)")
	flag.StringVar(&password, "password", "devpassword1", "web UI password for the user")
	flag.StringVar(&label, "label", "davlint", "app password label")
	flag.StringVar(&davlintCfg, "davlint-config", "./davlint.yaml", "path to davlint.yaml to update with the generated token")
	flag.Parse()

	if name == "" {
		name = username
	}

	ctx := context.Background()

	db, err := sqlite.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close() //nolint:errcheck // best-effort on exit

	if err := migrations.Run(ctx, db.DB()); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	// Create the user if they don't already exist.
	user, err := db.GetUserByUsername(ctx, username)
	switch {
	case errors.Is(err, store.ErrNotFound):
		user, err = local.CreateUser(ctx, db, username, name, password, false)
		if err != nil {
			return fmt.Errorf("create user %q: %w", username, err)
		}
		fmt.Printf("created user:    %s (display: %q)\n", username, name)
	case err != nil:
		return fmt.Errorf("look up user: %w", err)
	default:
		fmt.Printf("user exists:     %s (id: %s)\n", username, user.ID)
	}

	// Generate a new app password token.
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Errorf("generate token: %w", err)
	}
	// base32 without padding for a clean token
	token := "chub_" + base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf)
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(token)))

	ap := &store.AppPassword{
		ID:        uuid.NewString(),
		UserID:    user.ID,
		Name:      label,
		TokenHash: hash,
		CreatedAt: time.Now().UTC(),
	}
	if err := db.CreateAppPassword(ctx, ap); err != nil {
		return fmt.Errorf("create app password: %w", err)
	}

	fmt.Printf("app password:    %s\n", token)

	if err := writeDavlintConfig(davlintCfg, username, token); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not update %s: %v\n", davlintCfg, err)
		fmt.Printf("\nPaste manually into %s:\n", davlintCfg)
		fmt.Printf("  principals:\n")
		fmt.Printf("    - username: %q\n", username)
		fmt.Printf("      password: %q\n", token)
	} else {
		fmt.Printf("wrote token →    %s\n", davlintCfg)
	}

	return nil
}

// writeDavlintConfig sets the password for the given username in the davlint
// YAML config at path. If the file does not exist it is created from
// <name>.example<ext> (e.g. davlint.example.yaml) in the same directory.
// Comments and formatting are preserved via the yaml.v3 node API.
func writeDavlintConfig(path, username, token string) error {
	// Bootstrap from the example file if the target doesn't exist yet.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		dir := filepath.Dir(path)
		base := filepath.Base(path)
		ext := filepath.Ext(base)
		stem := strings.TrimSuffix(base, ext)
		examplePath := filepath.Join(dir, stem+".example"+ext)

		data, readErr := os.ReadFile(examplePath) // #nosec G304 -- dev tool; path is derived from flag
		if os.IsNotExist(readErr) {
			return fmt.Errorf("%s not found and %s does not exist to bootstrap from", path, examplePath)
		}
		if readErr != nil {
			return fmt.Errorf("read %s: %w", examplePath, readErr)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil { // #nosec G703 G306 -- dev tool; path from flag, 0600 for credentials
			return fmt.Errorf("create %s: %w", path, err)
		}
	}

	data, err := os.ReadFile(path) // #nosec G304 -- dev tool; path is derived from flag
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	// Parse as a yaml.Node to preserve comments.
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	if len(doc.Content) == 0 {
		return fmt.Errorf("%s is empty", path)
	}

	if err := setYAMLPrincipalPassword(doc.Content[0], username, token); err != nil {
		return err
	}

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	return os.WriteFile(path, out, 0o600) // #nosec G306 -- 0600 appropriate for credential file
}

// setYAMLPrincipalPassword walks the root mapping node of a davlint config and
// sets the password scalar for the named username under the principals sequence.
func setYAMLPrincipalPassword(root *yaml.Node, username, token string) error {
	// root is a MappingNode: [key, value, key, value, ...]
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value != "principals" {
			continue
		}
		seq := root.Content[i+1] // SequenceNode
		for _, item := range seq.Content {
			// item is a MappingNode for one principal entry
			var foundUser bool
			for j := 0; j+1 < len(item.Content); j += 2 {
				if item.Content[j].Value == "username" && item.Content[j+1].Value == username {
					foundUser = true
					break
				}
			}
			if !foundUser {
				continue
			}
			for j := 0; j+1 < len(item.Content); j += 2 {
				if item.Content[j].Value == "password" {
					item.Content[j+1].Value = token
					item.Content[j+1].Style = yaml.DoubleQuotedStyle
					return nil
				}
			}
		}
	}
	return fmt.Errorf("principal %q not found in %s", username, "davlint.yaml")
}
