Go-DVM

DVM engine to write DVMs on top.

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