package validation

import (
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
)

// New creates the Validation sub-agent.
func New(model model.LLM, sandboxTool tool.Tool) (agent.Agent, error) {
	instruction := `You are the Validation agent. Your sole responsibility is to dry-run and validate generated code (Python / C++) scripts before final delivery.
You have access to the "sandbox_tool" tool.
When provided with code, call the sandbox tool specifying the correct language ("python" or "cpp").
Analyze the tool's output:
- If "success" is true and there are no runtime exceptions, reply with exactly: "VALIDATION_SUCCESSFUL" followed by the code itself.
- If "success" is false, analyze the compilation/runtime errors. Attempt to rewrite the code to resolve the errors and re-run the validation.
- If you cannot resolve the error after 3 attempts, output the error details and declare "VALIDATION_FAILED".`

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
