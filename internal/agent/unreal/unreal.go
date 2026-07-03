package unreal

import (
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
)

// New creates the Unreal Engine C++ sub-agent.
func New(model model.LLM, vectorSearchTool tool.Tool) (agent.Agent, error) {
	instruction := `You are the Unreal C++ Agent. Your role is to generate idiomatic C++ source code modifications and level actor code additions for Unreal Engine games.
Your goal is to integrate existing .wav audio files into complex game level classes (such as ALevelScriptActor or custom AActor level actors) by attaching UAudioComponent and setting up trigger volume overlap handlers.
You have access to the "vector_search" tool to query multi-modal semantic context and locate existing base C++ implementations, header files, and asset paths in the repository.
Always query the vector search database first to inspect existing C++ class structures and sound asset locations.
Emit ONLY valid C++ code snippet additions designed to integrate the WAV asset cleanly into the target base C++ level class. Do not generate Python scripts.`

	return llmagent.New(llmagent.Config{
		Name:        "unreal_agent",
		Description: "Queries base C++ classes via Multi-Modal Vector Search and generates Unreal Engine C++ level code additions for integrating existing WAV files.",
		Instruction: instruction,
		Model:       model,
		Tools: []tool.Tool{
			vectorSearchTool,
		},
		OutputKey: "unreal_script",
	})
}

