package terminal

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestBuildWorkingDirectoryAndArguments(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cwd 'quotes' $dollar `ticks`\nnext")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	value := "literal ' \" $() `ticks` \\\nline"
	spec := Command{Executable: "/bin/sh", Args: []string{"-c", `printf '%s\n%s\n%s' "$PWD" "$1" "${REMOVE-unset}"`, "--", value}, Dir: dir, Env: map[string]string{"EXPLICIT": "present"}, Unset: []string{"REMOVE"}}
	for _, term := range []string{"terminal", "iterm2", "ghostty", "custom --"} {
		t.Run(term, func(t *testing.T) {
			built, err := Build(term, spec)
			if err != nil {
				t.Fatal(err)
			}
			var cmd *exec.Cmd
			switch term {
			case "terminal", "iterm2":
				marker := " to do script "
				if term == "iterm2" {
					marker = " profile command "
				}
				_, literal, _ := strings.Cut(built.Args[2], marker)
				shell, err := strconv.Unquote(literal)
				if err != nil {
					t.Fatal(err)
				}
				cmd = exec.Command("/bin/sh", "-c", shell)
			case "ghostty":
				cmd = exec.Command("/bin/sh", "-c", built.Args[len(built.Args)-1])
			default:
				cmd = exec.Command(built.Args[2], built.Args[3:]...)
			}
			cmd.Env = append(os.Environ(), "REMOVE=terminal-value")
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%v: %s", err, output)
			}
			want := dir + "\n" + value + "\nunset"
			if string(output) != want {
				t.Fatalf("got %q want %q", output, want)
			}
		})
	}
}
func TestCommandValidationAndEnvironment(t *testing.T) {
	for _, spec := range []Command{{}, {Executable: "agent", Env: map[string]string{"A": "b"}, Unset: []string{"A"}}, {Executable: "agent", Env: map[string]string{"bad=key": "x"}}} {
		if _, err := Build("terminal", spec); err == nil {
			t.Fatalf("invalid spec accepted: %+v", spec)
		}
	}
	if _, err := Build("", Command{Executable: "agent"}); err == nil {
		t.Fatal("empty terminal accepted")
	}
	t.Setenv("INHERITED_SECRET", "secret")
	t.Setenv("REMOVE", "gone")
	spec := Command{Executable: "agent", Args: []string{"a b"}, Env: map[string]string{"Z": "last", "A": "first"}, Unset: []string{"REMOVE"}}
	args, err := spec.Argv()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(args, " "), "secret") || strings.Join(args, " ") != "/usr/bin/env -u REMOVE A=first Z=last agent a b" {
		t.Fatalf("argv: %q", args)
	}
	env := strings.Join(spec.Environment(), "\n")
	if !strings.Contains(env, "INHERITED_SECRET=secret") || strings.Contains(env, "REMOVE=") {
		t.Fatalf("incorrect inherited environment")
	}
}
