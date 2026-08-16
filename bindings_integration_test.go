package vtui

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestIntegration_PythonBindings(t *testing.T) {
	py, err := exec.LookPath("python3")
	if err != nil {
		py, err = exec.LookPath("python")
	}
	if err != nil {
		t.Skip("Python not found on this system. To test Python bindings, install Python: 'sudo apt install python3' (Debian/Ubuntu) or 'brew install python3' (macOS)")
	}

	testDir := filepath.Join("bindings", "python", "tests")
	if _, err := os.Stat(testDir); err != nil {
		t.Skipf("Python test directory %s not found", testDir)
	}

	pyDir, _ := filepath.Abs(filepath.Join("bindings", "python"))
	cmd := exec.Command(py, "-m", "unittest", "discover", "-s", testDir)
	cmd.Env = append(os.Environ(), "PYTHONPATH="+pyDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Python bindings test failed: %v\nOutput:\n%s", err, string(out))
	}
}

func TestIntegration_NodeBindings(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js not found on this system. To test Node bindings, install Node: 'sudo apt install nodejs' (Debian/Ubuntu) or 'brew install node' (macOS)")
	}

	testFile := filepath.Join("bindings", "node", "test", "test.js")
	if _, err := os.Stat(testFile); err != nil {
		t.Skipf("Node test file %s not found", testFile)
	}

	cmd := exec.Command(node, testFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Node.js bindings test failed: %v\nOutput:\n%s", err, string(out))
	}
}
func TestIntegration_CBindingsCompilation(t *testing.T) {
	cc, err := exec.LookPath("gcc")
	if err != nil {
		cc, err = exec.LookPath("clang")
	}
	if err != nil {
		t.Skip("C compiler (gcc/clang) not found on this system. Install: 'sudo apt install gcc' or 'xcode-select --install'")
	}

	tmpDir := t.TempDir()
	objPath := filepath.Join(tmpDir, "vtui.o")

	cmdCC := exec.Command(cc,
		"-c",
		"-I"+filepath.Join("bindings", "c", "include"),
		filepath.Join("bindings", "c", "src", "vtui.c"),
		"-o", objPath,
	)
	out, err := cmdCC.CombinedOutput()
	if err != nil {
		t.Fatalf("C compilation failed: %v\nOutput:\n%s", err, string(out))
	}
}

func TestIntegration_CppBindingsCompilation(t *testing.T) {
	cxx, err := exec.LookPath("g++")
	if err != nil {
		cxx, err = exec.LookPath("clang++")
	}
	if err != nil {
		t.Skip("C++ compiler (g++/clang++) not found on this system. Install: 'sudo apt install g++' or 'xcode-select --install'")
	}

	cmdCXX := exec.Command(cxx,
		"-fsyntax-only",
		"-std=c++17",
		"-I"+filepath.Join("bindings", "c", "include"),
		"-I"+filepath.Join("bindings", "cpp", "include"),
		filepath.Join("bindings", "cpp", "examples", "hello.cpp"),
	)
	out, err := cmdCXX.CombinedOutput()
	if err != nil {
		t.Fatalf("C++ syntax check failed: %v\nOutput:\n%s", err, string(out))
	}
}
