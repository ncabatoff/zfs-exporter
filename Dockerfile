# **** NOTE ****
# This doesn't yet yield a usable docker image.  I can't seem to get libzfs to initialize,
# even if I bind /dev/zfs and /dev/zvols in.  I'm including it anyway because it took time
# to get it to this point and I may return to the struggle at some point in the future.

# Start from a Debian image with the latest version of Go installed
# and a workspace (GOPATH) configured at /go.
FROM golang

# Build the zfs-exporter command inside the container.

#add the contrib repo to install the ZFS libs
RUN echo "deb http://ftp.debian.org/debian jessie-backports main contrib" >> /etc/apt/sources.list.d/backports.list

RUN apt-get update
RUN apt-get install lsb-release


#Use debian libdev pkg to replace the 404'ed ZoL pkg
RUN apt-get install --yes libzfslinux-dev

RUN dpkg --configure -a


RUN go get github.com/ncabatoff/go-libzfs github.com/prometheus/client_golang/prometheus

# Copy the local package files to the container's workspace.
ADD zfs-exporter /go/src/github.com/ncabatoff/zfs-exporter

RUN go install github.com/ncabatoff/zfs-exporter

USER root

# Run the zfs-exporter command by default when the container starts.
ENTRYPOINT /go/bin/zfs-exporter

# Document that the service listens on port 9254.
EXPOSE 9254
