#!/bin/sh
set -e

image="$1"
out="$2"
arch="$3"
url="http://archive.ubuntu.com/ubuntu"

[ "$arch" != "amd64" ] && [ "$arch" != "i386" ] && url="http://ports.ubuntu.com/ubuntu-ports"
sudo distrobuilder build-lxd "$image" "$out" --type split --compression xz \
    -o "image.architecture=$arch" \
    -o "source.url=$url"
