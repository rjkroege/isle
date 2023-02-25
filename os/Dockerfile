# Dockerfile to build the Guest image.
FROM alpine

# TODO(rjk): configure for the processor arch.
RUN apk --no-cache add \
        curl \
	squashfs-tools

# Set me up.
ENV initrd=https://github.com/lab47/isle-kernel/releases/download/v20220327/initrd-aarch64
ENV kernel=https://github.com/lab47/isle-kernel/releases/download/v20220327/linux-5.16.17-arm64.tar.xz
ENV root=https://dl-cdn.alpinelinux.org/alpine/v3.15/releases/aarch64/alpine-minirootfs-3.15.2-aarch64.tar.gz

WORKDIR /wd
WORKDIR /wd/release

# Fetch the initial ram disk.
RUN curl -o initrd -L $initrd

WORKDIR /wd/tmp

# Fetch the filesystem and kernel image.
RUN  curl -o rootfs.tar.gz -L $root
RUN curl -o kernel.tar.xz -L $kernel

WORKDIR /wd/rootfs

# Uncompress the root
RUN tar xf /wd/tmp/rootfs.tar.gz

# Copy the custom directory into the root.
COPY custom /wd/rootfs

# Need to copy resolve.conf into the image.
COPY resolv.conf /wd/rootfs/etc/resolv.conf
# Update the Alpine image.
RUN /usr/sbin/chroot /wd/rootfs /sbin/apk add --no-cache
# Clean up
RUN rm  /wd/rootfs/etc/resolv.conf

# Add passwords.
RUN sh -c "echo \"macstorage:x:147:100:For external file access,,,:/tmp:/sbin/nologin\" >> \"/wd/rootfs/etc/passwd\""
RUN sh -c "echo \"macstorage:!::0:::::\" >> \"/wd/rootfs/etc/shadow\""

# Add the special guest apps.
COPY isle-helper /wd/rootfs/usr/bin
COPY isle-guest /wd/rootfs/usr/sbin

# Unpack and prep the kernel.
RUN tar -C /wd/rootfs -xf /wd/tmp/kernel.tar.xz 

# Debugging
# RUN ls -lR  /wd/rootfs/boot/vmlinuz-*
# RUN which gunzip
# RUN gunzip --help

WORKDIR /wd/rootfs/boot

# RUN ls -l
# RUN ls -lR  /wd/rootfs/boot/vmlinuz-*
RUN gunzip -c vmlinuz-* > /wd/release/vmlinux
RUN rm -rf /wd/rootfs/boot/vmlinu*

# Install drivers
RUN KERNEL_VERSION=$(ls /wd/rootfs/lib/modules | head -n 1); \
	chroot /wd/rootfs /sbin/depmod -ae -F /boot/System.map-$KERNEL_VERSION $KERNEL_VERSION

# RUN ls -lR /wd/rootfs > /wd/logging.txt

# Build fs
RUN mksquashfs /wd/rootfs /wd/release/os.fs -comp xz

WORKDIR /wd
RUN tar -C /wd/release -czf os-unknown-arm64.tar.gz .

# At this point, we've finished and we can exfiltrate the os-unknown-arm64.tar.gz