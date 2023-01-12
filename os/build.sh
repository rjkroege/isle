# TODO(rjk): Can write this as a Go program. Or a Docker
# TODO(rjk): Make it more hermetic and incremental.
# TODO(rjk): uname isn't doing the right thing.
#if [ "$(uname -p)" = "aarch64" ]; then
  kernel=https://github.com/lab47/isle-kernel/releases/download/v20220327/linux-5.16.17-arm64.tar.xz
  root=https://dl-cdn.alpinelinux.org/alpine/v3.15/releases/aarch64/alpine-minirootfs-3.15.2-aarch64.tar.gz
  initrd=https://github.com/lab47/isle-kernel/releases/download/v20220327/initrd-aarch64
#else
#  kernel=https://github.com/lab47/isle-kernel/releases/download/v20220327/linux-5.16.17-x86.tar.xz
#  root=https://dl-cdn.alpinelinux.org/alpine/v3.15/releases/x86_64/alpine-minirootfs-3.15.2-x86_64.tar.gz
#  initrd=https://github.com/lab47/isle-kernel/releases/download/v20220327/initrd-x86_64
#fi

ROOT="${ROOT:-rootfs}"
TMP="${TMP:-/tmp}"
WD=`mktemp -d`

echo $ROOT $TMP $WD

# rm -rf "$ROOT"
mkdir -p "$WD/$ROOT"  || exit 1
mkdir -p "$WD"/release  || exit 1

echo "+ Downloading assets"

if ! test -e "$WD"/release/initrd; then
  curl -o "$WD"/release/initrd -L $initrd  || exit 1
fi

if ! test -e $TMP/rootfs.tar.gz; then
  curl -o $TMP/rootfs.tar.gz -L $root  || exit 1
fi

if ! test -e $TMP/kernel.tar.xz; then
  curl -o $TMP/kernel.tar.xz -L $kernel  || exit 1
fi

echo "+ Unpacking rootfs"
pushd "$WD/$ROOT"
sudo tar xf $TMP/rootfs.tar.gz || exit 1
popd

echo "+ Applying custom code"
sudo cp -a custom/* "$WD/$ROOT"  || exit 1

sudo chown root.root -R "$WD/$ROOT" || exit 1

echo "+ Add package"

sudo cp /etc/resolv.conf "$WD/$ROOT/etc/resolv.conf" || exit 1

sudo /usr/sbin/chroot "$WD/$ROOT" /sbin/apk add --no-cache || exit 1

sudo rm "$WD/$ROOT/etc/resolv.conf" || exit 1

echo "+ Add macstorage user"

sudo sh -c "echo \"macstorage:x:147:100:For external file access,,,:/tmp:/sbin/nologin\" >> \"$WD/$ROOT/etc/passwd\"" || exit 1
sudo sh -c "echo \"macstorage:!::0:::::\" >> \"$WD/$ROOT/etc/shadow\"" || exit 1

echo "+ Adding isle-guest"

sudo cp isle-guest "$WD/$ROOT/usr/sbin/" || exit 1

echo "+ Adding isle-helper"

sudo cp isle-helper "$WD/$ROOT/usr/bin/" || exit 1

echo "+ Adding kernel models"

sudo tar -C "$WD/$ROOT" -xf $TMP/kernel.tar.xz || exit 1

sudo gunzip < "$WD/$ROOT"/boot/vmlinuz-* > "$WD"/release/vmlinux || exit 1

# We don't use these and they just take up space.
sudo rm -rf "$WD/$ROOT"/boot/vmlinu* || exit 1

KERNEL_VERSION=$(ls $WD/$ROOT/lib/modules | head -n 1) || exit 1

echo "- Running depmod to be sure modules are ready"

sudo chroot "$WD/$ROOT" /sbin/depmod -ae -F /boot/System.map-$KERNEL_VERSION $KERNEL_VERSION || exit 1

sudo rm -f "$WD"/release/os.fs || exit 1
sudo mksquashfs "$WD/$ROOT" "$WD"/release/os.fs -comp xz || exit 1

# Move release into cwd so that we can tar it up.
sudo mv "$WD"/release . || exit 1

# TODO(rjk): Do something to fix up the naming.
