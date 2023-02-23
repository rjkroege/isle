#!/bin/sh

./bin/linux \
	--state-dir $home/wrks/islestate  \
	-i ghcr.io/rjkroege/debslim \
	-d /home/rjkroege/mac/tools/_builds/isle/os \
	 'sh guestbuild.sh'

# Convenience

# TODO(rjk): Make this independent of processor architecture.
# rm -f $ISLE_CACHE_DIR/os-unknown-arm64.tar.gz
# cp os.fs $ISLE_CACHE_DIR

# This is not very good.