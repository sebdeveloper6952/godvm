Go-DVM

A simple library to write DVMs using golang.

### Motivation
Every time I want to write a new DVM, I don't want to write the same stuff:
- relay logic
- event logic
- payment logic

### Objective
Make you focus on your DVM logic.

### Typical Request Flow Chart
![payment-flow](https://github.com/sebdeveloper6952/godvm/assets/18562903/73336930-475f-4714-bd10-b6c3f0739f62)

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
