package examples

import (
	"context"
	"log"

	goNostr "github.com/nbd-wtf/go-nostr"
	"github.com/sebdeveloper6952/godvm"
)

type simpleDVM struct {
	sk      string
	pk      string
	version string
	name    string
	about   string
	picture string
}

func main() {
	sk := "a19ad601202f0ef2ebc344a041676314ad812fbac1ff8410ede3163662847527" // don't reuse this private key
	pk, _ := goNostr.GetPublicKey(sk)

	dvm := &simpleDVM{
		sk:      sk,
		pk:      pk,
		version: "dvm-version-here", // refer to NIP-89 for d-tag versioning.
		name:    "My DVM Name",
		about:   "Example DVM that always returns the same image URL as result.",
		picture: "https://user-images.githubusercontent.com/99301796/223592277-34058d0e-af30-411d-8dfe-87c42dacdcf2.png",
	}

	engine, err := godvm.NewEngine()
	if err != nil {
		log.Fatal(err)
	}

	engine.RegisterDVM(dvm)

	engine.Run(
		context.TODO(),
		[]string{"wss://nostrue.com"},
	)
}

func (d *simpleDVM) PublicKeyHex() string {
	return d.pk
}

func (d *simpleDVM) Sign(e *goNostr.Event) error {
	return e.Sign(d.sk)
}

func (d *simpleDVM) Profile() *godvm.ProfileMetadata {
	return &godvm.ProfileMetadata{
		Name:    d.name,
		About:   d.about,
		Picture: d.picture,
	}
}

func (d *simpleDVM) KindSupported() int {
	return 5100 // image generation
}

func (d *simpleDVM) Version() string {
	return d.version
}

func (d *simpleDVM) acceptJob(input *godvm.Nip90Input) bool {
	return true
}

func (d *simpleDVM) Run(
	ctx context.Context,
	input *godvm.Nip90Input,
	chanToDvm <-chan *godvm.JobUpdate,
	chanToEngine chan<- *godvm.JobUpdate,
) bool {
	// return false here if after inspecting the input your DVM doesn't want to proceed with the job request.
	if !d.acceptJob(input) {
		return false
	}

	go func() {
		// signal godvm that you are processing the job request
		chanToEngine <- &godvm.JobUpdate{
			Status: godvm.StatusProcessing,
		}

		// The job is done, send a StatusSuccess message with the job result.
		// This finishes the cycle of the job request. After this, the channels must no longer be used.
		chanToEngine <- &godvm.JobUpdate{
			Status: godvm.StatusSuccess,
			Result: "https://user-images.githubusercontent.com/99301796/223592277-34058d0e-af30-411d-8dfe-87c42dacdcf2.png",
		}
	}()

	// return true to signal godvm that your dvm will proceed with the job request
	return true
}
