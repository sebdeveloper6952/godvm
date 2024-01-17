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
  - [] job request input output of another job
  - [] encrypted job params
  - [] parse job input marker to be forwarded to DVMs
  - [] support zaps
- remove logrus logger library, use standard "log"