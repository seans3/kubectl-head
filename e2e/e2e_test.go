package e2e

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

const (
	kindClusterName = "kubectl-head-e2e"
	testNamespace   = "e2e-test"
)

// TestMain sets up and tears down the Kind cluster.
func TestMain(m *testing.M) {
	if os.Getenv("E2E_TEST") == "" {
		fmt.Println("Skipping e2e tests. Set E2E_TEST to run.")
		return
	}

	fmt.Println("Setting up Kind cluster...")
	if err := setupCluster(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to set up Kind cluster: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	fmt.Println("Tearing down Kind cluster...")
	if err := teardownCluster(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to tear down Kind cluster: %v\n", err)
		// Don't exit with error, just log it.
	}

	os.Exit(code)
}

func setupCluster() error {
	cmd := exec.Command("kind", "create", "cluster", "--name", kindClusterName, "--wait", "5m")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create kind cluster: %w", err)
	}

	// Build the kubectl-head binary
	buildCmd := exec.Command("go", "build", "-o", "kubectl-head", "../cmd/kubectl-head")
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("failed to build kubectl-head binary: %w", err)
	}

	// Create a test namespace
	nsCmd := exec.Command("kubectl", "create", "namespace", testNamespace)
	if err := nsCmd.Run(); err != nil {
		return fmt.Errorf("failed to create namespace: %w", err)
	}

	return nil
}

func teardownCluster() error {
	cmd := exec.Command("kind", "delete", "cluster", "--name", kindClusterName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func TestHeadPods(t *testing.T) {
	// Create some pods for testing
	for i := 0; i < 10; i++ {
		podName := fmt.Sprintf("test-pod-%d", i)
		createPodCmd := exec.Command("kubectl", "run", podName, "--image=nginx", "-n", testNamespace)
		if err := createPodCmd.Run(); err != nil {
			t.Fatalf("Failed to create pod %s: %v", podName, err)
		}
	}

	// Wait for pods to be created
	time.Sleep(10 * time.Second)

	// Run the kubectl-head command
	headCmd := exec.Command("./kubectl-head", "pods", "-n", testNamespace, "--limit", "5")
	var out bytes.Buffer
	headCmd.Stdout = &out
	if err := headCmd.Run(); err != nil {
		t.Fatalf("kubectl-head command failed: %v", err)
	}

	// Verify the output
	output := out.String()
	t.Logf("kubectl-head output:\n%s", output)
	lines := strings.Split(output, "\n")
	var nonEmptyLines []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmptyLines = append(nonEmptyLines, line)
		}
	}

	// 5 pods + header + continue token
	if len(nonEmptyLines) != 7 {
		t.Errorf("Expected 7 non-empty lines of output, but got %d", len(nonEmptyLines))
	}

	if !strings.Contains(output, "Continue Token:") {
		t.Error("Expected output to contain a continue token, but it did not")
	}
}
