# zfs-exporter
Prometheus metrics exporter for ZFS.  

It's much the same data you get from zpool status and zpool iostat, but in the form of Prometheus
metrics.  Run

```
curl -s http://localhost:9254/metrics
```

to see them.

Sample dashboard is available at [grafana.net](https://grafana.net/dashboards/328).

## Caveats

Currently doesn't handle pool changes gracefully, you'll have to kill and
restart it if you create/import or destroy/export any pools.

libzfs is not a stable or official interface, so this could break with any new ZFS release.

Requires root privileges on Linux.  For the security conscious, run it with -web.listen-address=localhost:9254.  

If you're not running prometheus on the same host, either use my
[exporter-proxy](https://github.com/ncabatoff/exporter-proxy) or write a little cronjob that does

```
curl -s http://localhost:9254/metrics > mydir/zfs.tmp && mv mydir/zfs.tmp mydir/zfs.prom
```

Configure node_exporter with `-collector.textfile.directory=mydir` and it will
publish the stats to allow remote scraping.

## See also

https://github.com/eliothedeman/zfs_exporter

Same idea, but implemented by parsing the output of zpool using github.com/mistifyio/go-zfs.

https://github.com/eripa/prometheus-zfs

Ditto but without the dependency on go-zfs.

https://github.com/prometheus/node_exporter/pull/213

PR for node_exporter to add support for ZFS metrics.  Currently also relies on shelling out, 
which is apparently not allowed in node_exporter, so not clear where it's going. 

## Testing

To build and run zfs-exporter in a virtual environment, you need to install
VirtualBox, Vagrant, and Ansible 2.2+.  Then

```
platform=jessie # or fedora25
sudo ansible-galaxy install gantsign.golang
cd zfs-vagrant-ansible/
vagrant up $platform
cd ..
ANSIBLE_HOST_KEY_CHECKING=False ansible-playbook -i zfs-vagrant-ansible/.vagrant/provisioners/ansible/inventory/vagrant_ansible_inventory playbook-zfs-exporter.yml -l $platform
```

