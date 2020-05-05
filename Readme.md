
Original code taken from https://github.com/blacktop/docker-zeek, specifically https://github.com/blacktop/docker-zeek/tree/master/zeekctl

# TODO
- You need to have a valid node.cfg file. This would be a great place for a helper script.
- Use ethtool to turn off options for performance.
  - https://github.com/ncsa/bro-interface-setup
  - https://github.com/Security-Onion-Solutions/securityonion-setup/blob/ec219e2cbf72ffa52c4612e642e543c022f9c5ca/bin/sosetup-network#L446
- Killing the container early doesn't rotate the log files. Possible workaround is making sure the spool directory is mapped to the host. This won't gzip it but at least the logs will be preserved.
- Install zpkg