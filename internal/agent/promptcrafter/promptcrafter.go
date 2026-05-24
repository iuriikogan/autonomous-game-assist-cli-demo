package promptcrafter

import (
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
)

// New creates the Prompt Crafter sub-agent.
func New(model model.LLM) (agent.Agent, error) {
	instruction := `You are the Prompt Crafter agent. Your sole purpose is to expand a high-level user request into a highly descriptive foley / sound descriptor text.
Focus on:
- Physical sound attributes: texture, speed, impact, resonance.
- Atmospheric/acoustic dynamics: echo, reverb, space.
- Synths, real recordings, or physical foley methods to simulate the sound.
Emit ONLY the final expanded sound description. Do not include any prefix, conversational pleasantries, or meta-text.`

	return llmagent.New(llmagent.Config{
		Name:        "prompt_crafter",
		Description: "Expands short, abstract user prompts into rich physical sound and foley acoustic descriptors.",
		Instruction: instruction,
		Model:       model,
		OutputKey:   "foley_description",
	})
}
