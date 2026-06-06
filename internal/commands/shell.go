package commands

import (
	"bufio"
	"fmt"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
)

func newShellCommand(state *rootState) *cobra.Command {
	return &cobra.Command{
		Use:   "shell",
		Short: "Start an interactive VulnSky shell",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInteractiveShell(cmd, state)
		},
	}
}

func runInteractiveShell(cmd *cobra.Command, state *rootState) error {
	scanner := bufio.NewScanner(cmd.InOrStdin())
	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()
	contextPath := []string{}

	for {
		fmt.Fprint(out, shellPrompt(contextPath))
		if !scanner.Scan() {
			return scanner.Err()
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		fields, err := splitShellFields(line)
		if err != nil {
			fmt.Fprintf(errOut, "error: %v\n", err)
			continue
		}
		if len(fields) == 0 {
			continue
		}

		action, args := shellAction(contextPath, fields)
		switch action {
		case shellExit:
			return nil
		case shellSetContext:
			contextPath = args
		case shellRun:
			if err := executeShellCommand(cmd, state, args); err != nil {
				fmt.Fprintf(errOut, "error: %v\n", err)
			}
		}
	}
}

type shellActionKind int

const (
	shellRun shellActionKind = iota
	shellSetContext
	shellExit
)

func shellAction(contextPath []string, fields []string) (shellActionKind, []string) {
	head := fields[0]
	switch head {
	case "exit", "quit":
		return shellExit, nil
	case "back", "..":
		return shellSetContext, nil
	case "cd":
		if len(fields) == 1 || fields[1] == "/" || fields[1] == ".." {
			return shellSetContext, nil
		}
		if len(fields) == 2 && isShellContext(fields[1]) {
			return shellSetContext, []string{fields[1]}
		}
		return shellRun, append([]string{}, fields...)
	case "help", "?":
		if len(contextPath) == 0 {
			return shellRun, []string{"--help"}
		}
		return shellRun, append(append([]string{}, contextPath...), "--help")
	}

	if len(contextPath) == 0 && isShellContext(head) && len(fields) == 1 {
		return shellSetContext, []string{head}
	}
	if len(contextPath) > 0 && isRootCommand(head) {
		return shellRun, append([]string{}, fields...)
	}
	if len(contextPath) > 0 {
		return shellRun, append(append([]string{}, contextPath...), fields...)
	}
	return shellRun, append([]string{}, fields...)
}

func executeShellCommand(parent *cobra.Command, state *rootState, args []string) error {
	child := NewRootCommandWithOptions(RootOptions{
		RootDir:   state.rootDir,
		Profile:   state.profile,
		Factories: state.factories,
	})
	child.SetIn(parent.InOrStdin())
	child.SetOut(parent.OutOrStdout())
	child.SetErr(parent.ErrOrStderr())
	child.SetArgs(args)
	return child.Execute()
}

func shellPrompt(contextPath []string) string {
	if len(contextPath) == 0 {
		return "vulnsky/> "
	}
	return "vulnsky/" + strings.Join(contextPath, "/") + "> "
}

func isShellContext(value string) bool {
	switch value {
	case "oss", "image", "ecs", "profile", "records":
		return true
	default:
		return false
	}
}

func isRootCommand(value string) bool {
	switch value {
	case "completion", "deploy", "doctor", "ecs", "help", "image", "oss", "profile", "records", "shell":
		return true
	default:
		return false
	}
}

func splitShellFields(line string) ([]string, error) {
	fields := []string{}
	var current strings.Builder
	var quote rune
	inField := false

	for _, r := range line {
		if quote != 0 {
			if r == quote {
				quote = 0
				continue
			}
			current.WriteRune(r)
			inField = true
			continue
		}

		switch {
		case r == '\'' || r == '"':
			quote = r
			inField = true
		case unicode.IsSpace(r):
			if inField {
				fields = append(fields, current.String())
				current.Reset()
				inField = false
			}
		default:
			current.WriteRune(r)
			inField = true
		}
	}

	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote")
	}
	if inField {
		fields = append(fields, current.String())
	}
	return fields, nil
}
