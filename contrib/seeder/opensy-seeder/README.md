opensy-seeder
================

OpenSY-seeder is a crawler for the OpenSY network, which exposes a list
of reliable nodes via a built-in DNS server.

This is a fork of [bitcoin-seeder](https://github.com/sipa/bitcoin-seeder)
customized for the OpenSY blockchain.

Features
--------

* Regularly revisits known nodes to check their availability
* Bans nodes after enough failures, or bad behavior
* Accepts nodes down to v0.3.19 to request new IP addresses from,
  but only reports good post-v0.3.24 nodes.
* Keeps statistics over (exponential) windows of 2 hours, 8 hours,
  1 day and 1 week, to base decisions on
* Very low memory (a few tens of megabytes) and CPU requirements
* Crawlers run in parallel (by default 96 threads simultaneously)

Requirements
------------

* A Linux or macOS system
* A C++ compiler (GCC or Clang)
* OpenSSL development libraries

On Ubuntu/Debian:
```bash
sudo apt-get install build-essential libssl-dev
```

On macOS:
```bash
brew install openssl
export CXXFLAGS="-I/opt/homebrew/opt/openssl/include"
export LDFLAGS="-L/opt/homebrew/opt/openssl/lib"
```

Building
--------

```bash
make
```

This will produce the `dnsseed` binary.

Usage
-----

Assuming you want to run a DNS seed on `seed.opensy.net`, you will need:

1. A server with a static public IP address
2. Port 53 (UDP and TCP) open and not used by another service
3. Domain DNS configured to delegate to your server

### DNS Configuration

Add these records to your domain (e.g., in Cloudflare):

```
ns1.opensy.net.    IN  A      <your-server-ip>
seed.opensy.net.   IN  NS     ns1.opensy.net.
```

### Running the Seeder

```bash
# Basic usage
./dnsseed -h seed.opensy.net -n ns1.opensy.net -m admin@opensy.net

# With initial seed nodes
./dnsseed -h seed.opensy.net -n ns1.opensy.net -m admin@opensy.net \
    -s 192.168.1.100:9633 -s 192.168.1.101:9633

# For testnet
./dnsseed -h seed-testnet.opensy.net -n ns1.opensy.net \
    -m admin@opensy.net --testnet
```

### Command Line Options

```
-s <seed>       Seed node to collect peers from
-h <host>       Hostname of the DNS seed (e.g., seed.opensy.net)
-n <ns>         Hostname of the nameserver (e.g., ns1.opensy.net)
-m <mbox>       E-Mail address reported in SOA records
-t <threads>    Number of crawlers to run in parallel (default 96)
-d <threads>    Number of DNS server threads (default 4)
-a <address>    Address to listen on (default ::)
-p <port>       UDP port to listen on (default 53)
-o <ip:port>    Tor proxy IP/Port
--p2port <port> P2P port to connect to (default: 9633)
--testnet       Use testnet (port 19633)
--wipeban       Wipe list of banned nodes
--wipeignore    Wipe list of ignored nodes
```

### Running as a Service

Create `/etc/systemd/system/opensy-seeder.service`:

```ini
[Unit]
Description=OpenSY DNS Seeder
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/opensy-seeder
ExecStart=/opt/opensy-seeder/dnsseed -h seed.opensy.net -n ns1.opensy.net -m admin@opensy.net
Restart=always
RestartSec=30

[Install]
WantedBy=multi-user.target
```

Then enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable opensy-seeder
sudo systemctl start opensy-seeder
```

Network Parameters
------------------

| Parameter | Mainnet | Testnet |
|-----------|---------|---------|
| P2P Port | 9633 | 19633 |
| Magic Bytes | 0x53594c4d (SYLM) | 0x53594c54 (SYLT) |

Testing
-------

After starting the seeder, test with:

```bash
# Query your DNS seeder
dig seed.opensy.net @<your-server-ip>

# Should return IP addresses of discovered OpenSY nodes
```

Troubleshooting
---------------

### Port 53 already in use

On many Linux systems, `systemd-resolved` uses port 53. Disable it:

```bash
sudo systemctl stop systemd-resolved
sudo systemctl disable systemd-resolved

# Update DNS resolution
echo "nameserver 8.8.8.8" | sudo tee /etc/resolv.conf
```

### No nodes found

If the seeder finds no nodes, provide initial seeds:

```bash
./dnsseed -h seed.opensy.net -n ns1.opensy.net -m admin@opensy.net \
    -s <known-node-ip>:9633
```

License
-------

This software is released under the MIT license.

Based on bitcoin-seeder by Pieter Wuille (sipa).
Modified for OpenSY by the OpenSY developers.
