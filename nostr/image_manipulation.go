package nostr

import (
	"errors"
)

type ImageGenerationInput struct {
	Prompt         string
	Model          string
	Lora           string
	Ratio          string
	NegativePrompt string
	Size           string
}

func ImageGenerationInputFromNip90Input(i *Nip90Input) (*ImageGenerationInput, error) {
	// prompt is required
	if i.Input == "" {
		return nil, errors.New("prompt is required")
	}

	return &ImageGenerationInput{
		Prompt: i.Input,
	}, nil
}
