package main

import (
	"flag"
	"log"
	"net/http"
	_ "net/http/pprof"

	"github.com/bicomsystems/go-libzfs"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	zioTypeNames = []string{
		"Null",
		"Read",
		"Write",
		"Free",
		"Claim",
		"IoCtl",
	}

	vdevopsDesc = prometheus.NewDesc(
		"zfs_zpool_vdevops_total",
		"number of operations performed.",
		[]string{"poolname", "vdevtype", "vdevname", "vdevoptype"},
		nil)

	vdevbytesDesc = prometheus.NewDesc(
		"zfs_zpool_vdevbytes_total",
		"number of bytes handled",
		[]string{"poolname", "vdevtype", "vdevname", "vdevoptype"},
		nil)

	vdeverrorsDesc = prometheus.NewDesc(
		"zfs_zpool_errors_total",
		"number of errors seen",
		[]string{"poolname", "vdevtype", "vdevname", "errortype"},
		nil)

	vdevstateDesc = prometheus.NewDesc(
		"zfs_zpool_vdevstate",
		"vdev state: Unknown, Closed, Offline, Removed, CantOpen, Faulted, Degraded, Healthy.",
		[]string{"poolname", "vdevtype", "vdevname"},
		nil)

	vdevallocDesc = prometheus.NewDesc(
		"zfs_zpool_allocated_bytes",
		"number of bytes allocated (usage)",
		[]string{"poolname", "vdevtype", "vdevname"},
		nil)

	vdevspaceDesc = prometheus.NewDesc(
		"zfs_zpool_space_bytes",
		"size of the vdev in bytes (total capacity).",
		[]string{"poolname", "vdevtype", "vdevname"},
		nil)

	vdevfragDesc = prometheus.NewDesc(
		"zfs_zpool_fragmentation_percent",
		"device fragmentation percentage",
		[]string{"poolname", "vdevtype", "vdevname"},
		nil)

	poolstateDesc = prometheus.NewDesc(
		"zfs_zpool_poolstate",
		"pool state enum: Active, Exported, Destroyed, Spare, L2cache, uninitialized, unavail, potentiallyactive",
		[]string{"poolname"},
		nil)

	poolstatusDesc = prometheus.NewDesc(
		"zfs_zpool_poolstatus",
		"pool status enum: CorruptCache, MissingDevR, MissingDevNr, CorruptLabelR, CorruptLabelNr, BadGUIDSum, CorruptPool, CorruptData, FailingDev, VersionNewer, HostidMismatch, IoFailureWait, IoFailureContinue, BadLog, Errata, UnsupFeatRead, UnsupFeatWrite, FaultedDevR, FaultedDevNr, VersionOlder, FeatDisabled, Resilvering, OfflineDev, RemovedDev, Ok",
		[]string{"poolname"},
		nil)
)

type (
	ZfsCollector struct{}
)

func init() {
	prometheus.MustRegister(&ZfsCollector{})
}

func main() {
	var (
		listenAddress = flag.String("web.listen-address", ":9254", "Address on which to expose metrics and web interface.")
		metricsPath   = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
	)
	flag.Parse()

	http.Handle(*metricsPath, prometheus.Handler())

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>ZFS Exporter</title></head>
			<body>
			<h1>ZFS Exporter</h1>
			<p><a href="` + *metricsPath + `">Metrics</a></p>
			</body>
			</html>`))
	})
	http.ListenAndServe(*listenAddress, nil)
}

func poolname(pool zfs.Pool) string {
	return pool.Properties[zfs.PoolPropName].Value
}

// Describe implements prometheus.Collector.
func (z *ZfsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- vdevopsDesc
	ch <- vdevbytesDesc
	ch <- vdeverrorsDesc
	ch <- vdevstateDesc
	ch <- vdevallocDesc
	ch <- vdevspaceDesc
	ch <- vdevfragDesc
	ch <- poolstateDesc
	ch <- poolstatusDesc
	// TODO add error metric
}

// Collect implements prometheus.Collector.
func (z *ZfsCollector) Collect(ch chan<- prometheus.Metric) {
	pools, err := zfs.PoolOpenAll()
	if err != nil {
		log.Fatal("error opening pools: %v", err)
	}

	for _, pool := range pools {
		ch <- prometheus.MustNewConstMetric(poolstateDesc,
			prometheus.GaugeValue,
			poolstate(pool),
			poolname(pool))

		ch <- prometheus.MustNewConstMetric(poolstatusDesc,
			prometheus.GaugeValue,
			poolstatus(pool),
			poolname(pool))

		vdt, err := pool.VDevTree()
		if err != nil {
			log.Printf("unable to read vdevtree for pool '%s': %v", poolname(pool), err)
		}
		visitVdevs(pool, vdt, func(pool zfs.Pool, vdt zfs.VDevTree) {
			poolName := poolname(pool)
			vType := string(vdt.Type)

			ch <- prometheus.MustNewConstMetric(vdevstateDesc, prometheus.GaugeValue,
				float64(vdt.Stat.State), poolName, vType, vdt.Name)
			ch <- prometheus.MustNewConstMetric(vdevallocDesc, prometheus.GaugeValue,
				float64(vdt.Stat.Alloc), poolName, vType, vdt.Name)
			ch <- prometheus.MustNewConstMetric(vdevspaceDesc, prometheus.GaugeValue,
				float64(vdt.Stat.Space), poolName, vType, vdt.Name)
			ch <- prometheus.MustNewConstMetric(vdevfragDesc, prometheus.GaugeValue,
				float64(vdt.Stat.Fragmentation), poolName, vType, vdt.Name)

			ch <- prometheus.MustNewConstMetric(vdeverrorsDesc, prometheus.CounterValue,
				float64(vdt.Stat.ReadErrors), poolName, vType, vdt.Name, "read")
			ch <- prometheus.MustNewConstMetric(vdeverrorsDesc, prometheus.CounterValue,
				float64(vdt.Stat.WriteErrors), poolName, vType, vdt.Name, "write")
			ch <- prometheus.MustNewConstMetric(vdeverrorsDesc, prometheus.CounterValue,
				float64(vdt.Stat.ChecksumErrors), poolName, vType, vdt.Name, "checksum")

			for optype := zfs.ZIOTypeRead; optype < zfs.ZIOTypes; optype++ {
				ch <- prometheus.MustNewConstMetric(vdevopsDesc, prometheus.CounterValue,
					float64(vdt.Stat.Ops[optype]),
					poolName, vType, vdt.Name, zioTypeNames[optype])
			}

			for optype := zfs.ZIOTypeRead; optype < zfs.ZIOTypes; optype++ {
				ch <- prometheus.MustNewConstMetric(vdevbytesDesc, prometheus.CounterValue,
					float64(vdt.Stat.Bytes[optype]),
					poolName, vType, vdt.Name, zioTypeNames[optype])
			}
		})

		pool.Close()
	}

}

func poolstatus(pool zfs.Pool) float64 {
	pstatus, err := pool.Status()
	if err != nil {
		log.Printf("error getting status of pool '%s': %v\n", poolname(pool), err)
		return -1
	}
	return float64(pstatus)
}

func poolstate(pool zfs.Pool) float64 {
	pstate, err := pool.State()
	if err != nil {
		log.Printf("error getting state of pool '%s': %v\n", poolname(pool), err)
		return -1
	}
	return float64(pstate)
}

func visitVdevs(pool zfs.Pool, vdt zfs.VDevTree, visitor func(pool zfs.Pool, vdt zfs.VDevTree)) {
	visitor(pool, vdt)
	for _, child := range vdt.Devices {
		visitVdevs(pool, child, visitor)
	}
}
