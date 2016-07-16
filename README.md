# zfs-exporter
Prometheus metrics exporter for ZFS.  

It's much the same data you get from zpool status and zpool iostat, but in the form of Prometheus
metrics.  Run

  curl http://localhost:9054/metrics

to see them.

Caveats:

Currently doesn't handle pool changes gracefully, you'll have to kill and
restart it if you create/import or destroy/export any pools.

libzfs is not a stable or official interface, so this could break with any new ZFS release.
