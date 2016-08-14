# **** NOTE ****
# This doesn't yet yield a usable docker image.  I can't seem to get libzfs to initialize,
# even if I bind /dev/zfs and /dev/zvols in.  I'm including it anyway because it took time
# to get it to this point and I may return to the struggle at some point in the future.

# Start from a Debian image with the latest version of Go installed
# and a workspace (GOPATH) configured at /go.
FROM golang

# Build the zfs-exporter command inside the container.
RUN apt-get update
RUN apt-get install lsb-release
RUN wget http://archive.zfsonlinux.org/debian/pool/main/z/zfsonlinux/zfsonlinux_6_all.deb
RUN dpkg -i zfsonlinux_6_all.deb
RUN apt-get update
RUN apt-get --yes install libzfs-dev
RUN go get github.com/ncabatoff/go-libzfs github.com/prometheus/client_golang/prometheus 

# Copy the local package files to the container's workspace.
ADD zfs-exporter /go/src/github.com/ncabatoff/zfs-exporter

RUN go install github.com/ncabatoff/zfs-exporter

USER root

# Run the zfs-exporter command by default when the container starts.
ENTRYPOINT /go/bin/zfs-exporter

# Document that the service listens on port 9254.
EXPOSE 9254
