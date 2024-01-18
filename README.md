Go-DVM

DVM engine to write DVMs on top.

TODO List
- [] fully implement nip-90
  - [x] add job kinds 
  - [x] advertise DVMs nip-89
  - [x] job request relay list: publish job feedback, result, to list of relays
  - [x] job request input as text
  - [x] job request input as event
  - [x] job request input as url
  - [x] parse job input marker to be forwarded to DVMs
  - [x] job request input output of another job
  - [] fix dvm advertisement (use same d tag)
  - [] wait for multiple events/jobs for input to dvm
  - [] encrypted job params
  - [] support zaps
- remove logrus logger library, use standard "log"