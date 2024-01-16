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
	if i.Input == "" {
		return nil, errors.New("prompt is required")
	}

	if _, err := url.ParseRequestURI(i.Input); err != nil {
		return nil, fmt.Errorf("invalid url %s %w", i.Input, err)
	}

	return &MalwareScanningInput{
		URL: i.Input,
	}, nil
}
