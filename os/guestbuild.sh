#!/bin/sh

# Runs in debslim OCI. Requires buildah to be installed in debslim.

sudo buildah rm mgcontainer
sudo buildah bud --layers=true  -t makeguest
sudo buildah from --name mgcontainer makeguest
mkg=`sudo buildah mount mgcontainer`
echo $mkg

sudo cp $mkg/wd/os-unknown-arm64.tar.gz  .
sudo cp $mkg/wd/logging.txt .
