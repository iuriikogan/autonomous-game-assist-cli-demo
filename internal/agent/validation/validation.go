package validation

import (
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
)

// New creates the Validation sub-agent.
func New(model model.LLM, sandboxTool tool.Tool) (agent.Agent, error) {
	instruction := `You are the Validation agent. Your sole responsibility is to validate generated C++ code modifications and verify Unreal Engine mock smoke tests before final delivery.
You have access to the "sandbox_tool" tool.
When provided with C++ level code additions or diff snippets, call the sandbox tool specifying language "cpp".
Analyze the tool's output:
- If "success" is true and all mock smoke tests pass, reply with exactly: "VALIDATION_SUCCESSFUL" followed by the validated C++ code.
- If "success" is false (compilation errors or smoke test failures), analyze the g++ errors or failed test assertions. Attempt to fix the C++ code and re-run validation via the sandbox tool.
- If you cannot resolve the error after 3 attempts, output the failure logs and declare "VALIDATION_FAILED".`

	return llmagent.New(llmagent.Config{
		Name:        "validation_agent",
		Description: "Validates generated code by executing it in the sandboxed local subprocess and inspecting exit status/errors.",
		Instruction: instruction,
		Model:       model,
		Tools: []tool.Tool{
			sandboxTool,
		},
		OutputKey: "validated_script",
	})
}
