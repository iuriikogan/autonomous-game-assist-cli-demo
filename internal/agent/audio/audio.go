package audio

import (
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// New creates the Creative Audio multimodal synthesis sub-agent.
func New(model model.LLM) (agent.Agent, error) {
	instruction := `You are the Creative Audio agent. Your job is to synthesize a high-fidelity WAV audio file natively using Gemini's multimodal audio outputs.
You will be provided with a rich foley description produced by Prompt Crafter.
Synthesize and output ONLY the binary audio data according to the requested sound dynamics (e.g. footsteps, rustling, explosion).`

	// Configure Gemini to emit WAV audio natively
	audioConfig := &genai.GenerateContentConfig{
		ResponseMIMEType:   "audio/wav",
		ResponseModalities: []string{"AUDIO"},
		SpeechConfig: &genai.SpeechConfig{
			VoiceConfig: &genai.VoiceConfig{
				PrebuiltVoiceConfig: &genai.PrebuiltVoiceConfig{
					VoiceName: "Aoede", // Prebuilt voice suitable for sound synthesis or speech
				},
			},
		},
	}

	return llmagent.New(llmagent.Config{
		Name:                  "creative_audio",
		Description:           "Natively synthesizes custom game sound effects (WAV format) using multimodal audio generation.",
		Instruction:           instruction,
		Model:                 model,
		GenerateContentConfig: audioConfig,
		OutputKey:             "audio_binary",
	})
}
