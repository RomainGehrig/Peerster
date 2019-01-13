# Reputation Part : Notes for the write-up

## Changes to TxPublish and their impact
- A user can download a chunk multiple times (before 5 seconds) and only use it's reputation once. However we don't care about this case as a received chunk is already present and only a malicious peer would behave like that (which isn't what we want here).