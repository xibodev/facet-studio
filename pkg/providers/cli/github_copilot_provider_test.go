package cliprovider

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	copilot "github.com/github/copilot-sdk/go"
)

func TestCopilotLoggedInUserEnvironmentRemovesTokenOverrides(t *testing.T) {
	for _, name := range []string{"COPILOT_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN"} {
		t.Setenv(name, "must-not-leak")
	}
	t.Setenv("FACET_COPILOT_TEST_VALUE", "preserved")

	environment := copilotLoggedInUserEnvironment()
	for _, entry := range environment {
		name, value, _ := strings.Cut(entry, "=")
		switch strings.ToUpper(name) {
		case "COPILOT_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN":
			t.Fatalf("token override %s leaked into Copilot subprocess environment", name)
		case "FACET_COPILOT_TEST_VALUE":
			if value != "preserved" {
				t.Fatalf("preserved environment value = %q", value)
			}
		}
	}

	if os.Getenv("GH_TOKEN") != "must-not-leak" {
		t.Fatal("test unexpectedly mutated parent environment")
	}
}

func TestLiveGitHubCopilotCatalog(t *testing.T) {
	if os.Getenv("FACET_TEST_LIVE_COPILOT_CATALOG") != "1" {
		t.Skip("set FACET_TEST_LIVE_COPILOT_CATALOG=1 for authenticated catalog verification")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	status, models, err := InspectGitHubCopilot(ctx)
	if err != nil {
		t.Fatalf("InspectGitHubCopilot() error = %v", err)
	}
	if status == nil || !status.IsAuthenticated {
		t.Fatal("Copilot CLI is not authenticated")
	}
	if len(models) == 0 {
		t.Fatal("Copilot CLI returned no models")
	}
	t.Logf("authenticated Copilot catalog contains %d models", len(models))
}

func TestGitHubCopilotChatRejectsStudioToolsBeforeInference(t *testing.T) {
	provider := &GitHubCopilotProvider{}
	_, err := provider.Chat(t.Context(), []Message{{Role: "user", Content: "hello"}}, []ToolDefinition{{Type: "function"}}, "gpt-4.1", nil)
	if err == nil || !strings.Contains(err.Error(), "does not yet support Studio tool calls") {
		t.Fatalf("Chat() error = %v", err)
	}
}

func TestGitHubCopilotChatRejectsSerializedHistoryBeforeInference(t *testing.T) {
	provider := &GitHubCopilotProvider{}
	_, err := provider.Chat(t.Context(), []Message{{Role: "user", Content: "one"}, {Role: "assistant", Content: "two"}}, nil, "gpt-4.1", nil)
	if err == nil || !strings.Contains(err.Error(), "one text-only user message") {
		t.Fatalf("Chat() error = %v", err)
	}
}

func TestGitHubCopilotTextSessionHasNoOnWireToolsOrHostContext(t *testing.T) {
	home := t.TempDir()
	capturePath := filepath.Join(t.TempDir(), "requests.jsonl")
	t.Setenv("FACET_STUDIO_HOME", home)
	t.Setenv("FACET_TEST_COPILOT_HELPER", "1")
	t.Setenv("FACET_TEST_COPILOT_CAPTURE", capturePath)

	client := newGitHubCopilotClient(copilot.StdioConnection{
		Path: os.Args[0],
		Args: []string{"-test.run=^TestGitHubCopilotFakeCLIProcess$", "--"},
	}, copilot.ModeEmpty)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("start fake Copilot CLI: %v", err)
	}
	t.Cleanup(client.ForceStop)

	session, err := client.CreateSession(ctx, githubCopilotTextSessionConfig("gpt-fixture"))
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if err := session.Disconnect(); err != nil {
		t.Fatalf("Disconnect() error = %v", err)
	}

	requests := readCopilotHelperRequests(t, capturePath)
	var create map[string]any
	for _, request := range requests {
		if request.Method == "session.create" {
			create = request.Params
			break
		}
	}
	if create == nil {
		t.Fatalf("session.create not captured: %#v", requests)
	}
	availableTools, ok := create["availableTools"].([]any)
	if !ok || len(availableTools) != 0 {
		t.Fatalf("availableTools = %#v, want an explicit empty list", create["availableTools"])
	}
	for _, name := range []string{"enableHostGitOperations", "enableSkills"} {
		if value, ok := create[name].(bool); !ok || value {
			t.Fatalf("%s = %#v, want false", name, create[name])
		}
	}
	if requestPermission, ok := create["requestPermission"].(bool); !ok || !requestPermission {
		t.Fatalf("requestPermission = %#v, want true", create["requestPermission"])
	}
	systemMessage, ok := create["systemMessage"].(map[string]any)
	if !ok {
		t.Fatalf("systemMessage = %#v", create["systemMessage"])
	}
	sections, ok := systemMessage["sections"].(map[string]any)
	if !ok {
		t.Fatalf("systemMessage.sections = %#v", systemMessage["sections"])
	}
	environmentContext, ok := sections["environment_context"].(map[string]any)
	if !ok || environmentContext["action"] != "remove" {
		t.Fatalf("environment_context = %#v, want remove", sections["environment_context"])
	}
}

type copilotHelperRequest struct {
	Method string         `json:"method"`
	Params map[string]any `json:"params"`
}

func readCopilotHelperRequests(t *testing.T, path string) []copilotHelperRequest {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fake Copilot capture: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	requests := make([]copilotHelperRequest, 0, len(lines))
	for _, line := range lines {
		var request copilotHelperRequest
		if err := json.Unmarshal([]byte(line), &request); err != nil {
			t.Fatalf("decode fake Copilot capture %q: %v", line, err)
		}
		requests = append(requests, request)
	}
	return requests
}

func TestGitHubCopilotFakeCLIProcess(t *testing.T) {
	if os.Getenv("FACET_TEST_COPILOT_HELPER") != "1" {
		return
	}
	capturePath := os.Getenv("FACET_TEST_COPILOT_CAPTURE")
	reader := bufio.NewReader(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)
	for {
		body, err := readCopilotHelperFrame(reader)
		if err != nil {
			if err == io.EOF {
				return
			}
			os.Exit(2)
		}
		if err := appendCopilotHelperCapture(capturePath, body); err != nil {
			os.Exit(3)
		}
		var request struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params map[string]any  `json:"params"`
		}
		if err := json.Unmarshal(body, &request); err != nil {
			os.Exit(4)
		}
		result := map[string]any{"success": true}
		switch request.Method {
		case "connect":
			result = map[string]any{"ok": true, "protocolVersion": 3, "version": "fixture"}
		case "session.create":
			result = map[string]any{
				"sessionId":     request.Params["sessionId"],
				"workspacePath": "",
			}
		}
		response, err := json.Marshal(map[string]any{
			"jsonrpc": "2.0",
			"id":      request.ID,
			"result":  result,
		})
		if err != nil {
			os.Exit(5)
		}
		if _, err := fmt.Fprintf(writer, "Content-Length: %d\r\n\r\n", len(response)); err != nil {
			os.Exit(6)
		}
		if _, err := writer.Write(response); err != nil {
			os.Exit(7)
		}
		if err := writer.Flush(); err != nil {
			os.Exit(8)
		}
	}
}

func readCopilotHelperFrame(reader *bufio.Reader) ([]byte, error) {
	contentLength := 0
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		name, value, found := strings.Cut(line, ":")
		if found && strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			contentLength, err = strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, err
			}
		}
	}
	if contentLength <= 0 {
		return nil, fmt.Errorf("missing Content-Length")
	}
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(reader, body); err != nil {
		return nil, err
	}
	return body, nil
}

func appendCopilotHelperCapture(path string, body []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(body); err != nil {
		return err
	}
	_, err = file.Write([]byte("\n"))
	return err
}
