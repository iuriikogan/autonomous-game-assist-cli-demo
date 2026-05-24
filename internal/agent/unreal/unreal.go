package unreal

import (
	"google.golang.org/adk/agent/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
)

// New creates the Unreal Engine 5 sub-agent.
func New(model model.Model, vectorSearchTool tool.Tool) (agent.Agent, error) {
	instruction := `You are the Unreal Agent. Your role is to generate Unreal Engine 5 (UE5) Python automation scripts.
These scripts will be used to link custom assets (sound files, materials, actors) into the Level Blueprint, coordinate trigger zones, or configure actors in the level.
You have access to the "vector_search" tool to query semantic context and locate existing Blueprint asset paths in the repository.
Always query the database first if you are unsure about class/asset names.
Emit ONLY the final, valid, raw Python code. Do not enclose it in markdown code blocks or add metadata.`

	return llmagent.New(llmagent.Config{
		Name:        "unreal_agent",
		Description: "Queries asset context via Vector Search and writes UE5 Python level integration/automation scripts.",
		Instruction: instruction,
		Model:       model,
		Tools: []tool.Tool{
			vectorSearchTool,
		},
	})
}
