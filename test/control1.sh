#!/usr/bin/env bash

set -e

bin/control -server localhost:5001 -cmd addlink -neighbor nodeB -neighbor-addr localhost:5002
bin/control -server localhost:5001 -cmd sendpacket -source-addr localhost:5001 -source nodeA -dest nodeB -payload "test message"