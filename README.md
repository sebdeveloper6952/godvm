- nostr service
  - relays, sub, pub events

- engine
  - runs loop that
    - receives events from nostr service and dispatches to DVMs
    - receives updates from DVMs and dispatches to nostr
    - talks to lightning service to get invoices
    - receives invoice state updates to inform DVMs
    - listens to zap events to inform DVMs

- dvm
  - each dvm has its own sk
  - interface that if implemented, creates a DVM
