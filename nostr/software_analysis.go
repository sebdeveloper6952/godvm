package nostr

import (
	"errors"
	"fmt"
	"net/url"
)

type MalwareScanningInput struct {
	URL string
}

func MalwareScanningInputFromNip90Input(i *Nip90Input) (*MalwareScanningInput, error) {
	if i.Inputs == nil || len(i.Inputs) != 1 {
		return nil, errors.New("must provide exactly 1 URL")
	}

	input := i.Inputs[0]

	if _, err := url.ParseRequestURI(input.Value); err != nil {
		return nil, fmt.Errorf("invalid url %s %w", input.Value, err)
	}

	if input.Type != "url" {
		return nil, fmt.Errorf("invalid input type %s", input.Value)
	}

	return &MalwareScanningInput{
		URL: i.Inputs[0].Value,
	}, nil
}
