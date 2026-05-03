package security

import (
	"fmt"
	"path/filepath"
	"strings"
)

// sensitiveHostPaths are directories that must never be bind-mounted into a sandbox.
var sensitiveHostPaths = []string{
	"/etc", "/proc", "/sys", "/dev", "/run", "/boot", "/root", "/var",
}

// forbiddenSbxFlags prevent container escape or host resource exposure (exact token match).
var forbiddenSbxFlags = map[string]bool{
	"--privileged":   true,
	"--network=host": true,
	"--pid=host":     true,
	"--ipc=host":     true,
	"--userns=host":  true,
}

// forbiddenFlagPrefixes catches --flag=value forms that escalate privilege.
var forbiddenFlagPrefixes = []string{
	"--cap-add=",
	"--device=",
	"--security-opt=",
	"--add-host=",
}

// ValidateSbxCommand validates that a stdio MCP command token slice is safe to execute.
//
// Accepted entry-point executables:
//
//   - "sbx"    — E2B sandbox CLI. Subcommands "exec" and "run" allowed.
//   - "docker" — Docker CLI. Only "run" subcommand allowed.
//
// Common rules for both:
//   - Dangerous flags (--privileged, --network=host, --pid=host, …) are rejected.
//   - Volume/mount paths are checked for path traversal and sensitive host dirs.
func ValidateSbxCommand(tokens []string) error {
	if len(tokens) == 0 {
		return fmt.Errorf("command is required for stdio transport")
	}

	switch tokens[0] {
	case "sbx":
		return validateSbxTokens(tokens)
	case "docker":
		return validateDockerTokens(tokens)
	default:
		return fmt.Errorf(
			"security violation: executable %q is not allowed; permitted executables: \"sbx\", \"docker\"",
			tokens[0],
		)
	}
}

func validateSbxTokens(tokens []string) error {
	if len(tokens) < 2 {
		return fmt.Errorf("security violation: sbx requires a subcommand (exec or run)")
	}
	switch tokens[1] {
	case "exec", "run":
	default:
		return fmt.Errorf(
			"security violation: sbx subcommand %q is not allowed; use \"exec\" or \"run\"",
			tokens[1],
		)
	}
	if len(tokens) < 3 {
		return fmt.Errorf(
			"security violation: sbx %s requires at least one argument (image or command)",
			tokens[1],
		)
	}
	return validateSbxArguments(tokens[2:])
}

func validateDockerTokens(tokens []string) error {
	if len(tokens) < 2 || tokens[1] != "run" {
		return fmt.Errorf("security violation: docker subcommand must be \"run\"")
	}
	if len(tokens) < 3 {
		return fmt.Errorf("security violation: docker run requires an image argument")
	}
	return validateSbxArguments(tokens[2:])
}

// validateSbxArguments inspects all flags and arguments after "sbx exec/run".
func validateSbxArguments(args []string) error {
	for i, arg := range args {
		if forbiddenSbxFlags[arg] {
			return fmt.Errorf("security violation: sbx flag %q is forbidden", arg)
		}

		for _, prefix := range forbiddenFlagPrefixes {
			if strings.HasPrefix(arg, prefix) {
				return fmt.Errorf("security violation: sbx flag %q is forbidden", arg)
			}
		}

		// --mount <spec> or --mount=<spec>
		if arg == "--mount" && i+1 < len(args) {
			if err := validateMountSpec(args[i+1]); err != nil {
				return err
			}
		}
		if spec, ok := strings.CutPrefix(arg, "--mount="); ok {
			if err := validateMountSpec(spec); err != nil {
				return err
			}
		}

		// -v <spec> or -v=<spec>  (short volume syntax)
		if arg == "-v" && i+1 < len(args) {
			if err := validateVolumeSpec(args[i+1]); err != nil {
				return err
			}
		}
		if spec, ok := strings.CutPrefix(arg, "-v="); ok {
			if err := validateVolumeSpec(spec); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateMountSpec validates a --mount type=bind,source=X,target=Y specification.
func validateMountSpec(spec string) error {
	if strings.Contains(spec, "..") {
		return fmt.Errorf("security violation: mount spec %q contains path traversal", spec)
	}
	for part := range strings.SplitSeq(spec, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		if key := strings.TrimSpace(kv[0]); key == "source" || key == "src" {
			if err := validateHostPath(strings.TrimSpace(kv[1])); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateVolumeSpec validates a -v /host:/container[:options] specification.
func validateVolumeSpec(spec string) error {
	if strings.Contains(spec, "..") {
		return fmt.Errorf("security violation: volume spec %q contains path traversal", spec)
	}
	parts := strings.SplitN(spec, ":", 3)
	if len(parts) < 2 {
		return nil // anonymous volume — no host path involved
	}
	return validateHostPath(parts[0])
}

// validateHostPath rejects mounts of sensitive host directories.
func validateHostPath(hostPath string) error {
	clean := filepath.Clean(hostPath)
	for _, sensitive := range sensitiveHostPaths {
		if clean == sensitive || strings.HasPrefix(clean, sensitive+"/") {
			return fmt.Errorf(
				"security violation: mounting host path %q is forbidden (sensitive system directory)",
				hostPath,
			)
		}
	}
	return nil
}
