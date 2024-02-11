package godvm

import (
	"context"

	goNostr "github.com/nbd-wtf/go-nostr"
)

type Dvmer interface {
	// PublicKeyHex return the public key of the DVM in hex format.
	PublicKeyHex() string

	// KindSupported returns the job request kind that the DVM can handle.
	KindSupported() int

	// Version returns a string that is used as a d-tag for NIP-89 publishing.
	// Refer to NIP-89: https://github.com/nostr-protocol/nips/blob/protected-events-tag/89.md
	Version() string

	// Profile returns the profile information of the DVM. This is used to publish both kind-0 and NIP-89 application
	// handler events.
	Profile() *ProfileMetadata

	// Sign signs the given event with the private key of the DVM.
	Sign(e *goNostr.Event) error

	// Run executes the DVM main logic. The input comes directly from the nostr job request event.
	// The return value must be `true` if your DVM wants to proceed with the job, else return `false`.
	// If your DVM proceeds with the job, use the provided channels to communicate back and forth with the library.
	// See the examples/ directory for a better explanation with code.
	Run(
		ctx context.Context,
		input *Nip90Input,
		chanToDvm <-chan *JobUpdate,
		chanToEngine chan<- *JobUpdate,
	) bool
}
