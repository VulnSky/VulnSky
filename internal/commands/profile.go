package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"vulnsky/internal/config"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

func newProfileCommand(state *rootState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "ls",
		Short: "List profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfileList(cmd, state)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "show [profile]",
		Short: "Show profile values",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := activeProfileName(state)
			if err != nil {
				return err
			}
			if len(args) > 0 {
				name = args[0]
			}
			return runProfileShow(cmd, state, name)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "use <profile>",
		Short: "Set active profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfileUse(cmd, state, args[0])
		},
	})
	return cmd
}

func runProfileList(cmd *cobra.Command, state *rootState) error {
	active, err := activeProfileName(state)
	if err != nil {
		return err
	}
	names, err := listProfileNames(state.rootDir)
	if err != nil {
		return err
	}
	for _, name := range names {
		mark := " "
		if name == active {
			mark = "*"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", mark, name)
	}
	return nil
}

func runProfileShow(cmd *cobra.Command, state *rootState, name string) error {
	path, err := profileEnvPath(state.rootDir, name)
	if err != nil {
		return err
	}
	values, err := readProfileEnv(path)
	if err != nil {
		return err
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	fmt.Fprintf(cmd.OutOrStdout(), "profile=%s\npath=%s\n", name, path)
	for _, key := range keys {
		value := values[key]
		if isSecretKey(key) {
			value = "<redacted>"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s=%s\n", key, value)
	}
	return nil
}

func runProfileUse(cmd *cobra.Command, state *rootState, name string) error {
	path, err := profileEnvPath(state.rootDir, name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("profile %q not found: %s", name, path)
	}
	if _, err := readProfileEnv(path); err != nil {
		return err
	}
	envPath := filepath.Join(state.rootDir, ".env")
	if err := upsertEnvValue(envPath, "VULNSKY_ACTIVE_PROFILE", name); err != nil {
		return err
	}
	state.profile = name
	fmt.Fprintf(cmd.OutOrStdout(), "active profile=%s\n", name)
	return nil
}

func listProfileNames(rootDir string) ([]string, error) {
	profileDir, err := profileDirectory(rootDir)
	if err != nil {
		return nil, err
	}
	matches, err := filepath.Glob(filepath.Join(profileDir, "*.env"))
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(matches))
	for _, match := range matches {
		base := filepath.Base(match)
		names = append(names, strings.TrimSuffix(base, ".env"))
	}
	sort.Strings(names)
	return names, nil
}

func profileEnvPath(rootDir string, name string) (string, error) {
	if err := config.ValidateProfileName(name); err != nil {
		return "", err
	}
	profileDir, err := profileDirectory(rootDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(profileDir, name+".env"), nil
}

func profileDirectory(rootDir string) (string, error) {
	global, err := readRootEnv(rootDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(rootDir, firstNonEmpty(global["VULNSKY_PROFILE_DIR"], "./profiles")), nil
}

func activeProfileName(state *rootState) (string, error) {
	if state.profile != "" {
		if err := config.ValidateProfileName(state.profile); err != nil {
			return "", err
		}
		return state.profile, nil
	}
	global, err := readRootEnv(state.rootDir)
	if err != nil {
		return "", err
	}
	name := firstNonEmpty(global["VULNSKY_ACTIVE_PROFILE"], "default")
	if err := config.ValidateProfileName(name); err != nil {
		return "", err
	}
	return name, nil
}

func readRootEnv(rootDir string) (map[string]string, error) {
	path := filepath.Join(rootDir, ".env")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	values, err := godotenv.Read(path)
	if err != nil {
		return nil, fmt.Errorf("load env file %s: %w", path, err)
	}
	return values, nil
}

func readProfileEnv(path string) (map[string]string, error) {
	values, err := godotenv.Read(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("profile not found: %s", path)
		}
		return nil, fmt.Errorf("load profile %s: %w", path, err)
	}
	return values, nil
}

func upsertEnvValue(path string, key string, value string) error {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	lines := []string{}
	if len(data) > 0 {
		lines = strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	}
	replacement := key + "=" + value
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, key+"=") {
			lines[i] = replacement
			found = true
		}
	}
	if !found {
		lines = append(lines, replacement)
	}
	if len(lines) == 0 || strings.TrimSpace(lines[len(lines)-1]) != "" {
		lines = append(lines, "")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}

func isSecretKey(key string) bool {
	upper := strings.ToUpper(key)
	return strings.Contains(upper, "SECRET") ||
		strings.Contains(upper, "TOKEN") ||
		strings.Contains(upper, "PASSWORD")
}
