Go-DVM

A simple library to write DVMs using golang.

### Motivation
Every time I want to write a new DVM, I don't want to write the same stuff:
- relay logic
- event logic
- payment logic

### Objective
Make you focus on your DVM logic. 

### Features
- relay connection handling
- listen to job request events
- publish job feedback and result events
- publish kind `0` (Profile Metadata) and kind `31990` (NIP-89 Application Handler) events for discoverability of your DVM.

Refer to [NIP-90](https://github.com/nostr-protocol/nips/blob/master/90.md) for more information.

### Example
Refer to the examples folder for complete code examples.

TODO
- [ ] fully implement nip-90
  - [x] add job kinds 
  - [x] advertise DVMs nip-89
  - [x] job request relay list: publish job feedback, result, to list of relays
  - [x] job request input as text
  - [x] job request input as event
  - [x] job request input as url
  - [x] parse job input marker to be forwarded to DVMs
  - [x] job request input output of another job
  - [x] fix dvm advertisement (use same d tag)
  - [x] wait for multiple events/jobs for input to dvm
  - [x] include bid amount in input to DVMs so they can decide to accept/reject the job
  - [ ] encrypted job params
  - [ ] support zaps
