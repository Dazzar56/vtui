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
