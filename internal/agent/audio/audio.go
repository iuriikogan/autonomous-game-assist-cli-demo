package audio

import (
	"iter"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/session"
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

// NewMock creates a mock Creative Audio agent that returns dummy audio data.
func NewMock() (agent.Agent, error) {
	maa := &mockAudioAgent{}
	base, err := agent.New(agent.Config{
		Name:        "creative_audio",
		Description: "MOCK: Synthesizes dummy WAV audio for testing.",
		Run:         maa.run,
	})
	if err != nil {
		return nil, err
	}
	maa.Agent = base
	return maa, nil
}

type mockAudioAgent struct {
	agent.Agent
}

func (m *mockAudioAgent) run(ictx agent.InvocationContext) iter.Seq2[*session.Event, error] {
	return func(yield func(*session.Event, error) bool) {
		sess := ictx.Session()

		// Set dummy WAV bytes (a minimal valid-ish WAV header + some silence)
		dummyWav := []byte("RIFF\x24\x00\x00\x00WAVEfmt \x10\x00\x00\x00\x01\x00\x01\x00\x22\x56\x00\x00\x22\x56\x00\x00\x01\x00\x08\x00data\x00\x00\x00\x00")
		sess.State().Set("audio_binary", dummyWav)

		evt := session.NewEvent(ictx.InvocationID())
		evt.Content = &genai.Content{
			Parts: []*genai.Part{
				{Text: "SUCCESS: (MOCK) Audio synthesized successfully with dummy data."},
			},
		}
		yield(evt, nil)
	}
}

