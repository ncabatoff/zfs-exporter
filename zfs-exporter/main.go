package main

import (
	"log"
	"net/http"
	"time"

	"github.com/ncabatoff/go-libzfs"
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

	vdevops = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "zfs",
		Subsystem: "zpool",
		Name:      "vdevops_total",
		Help:      "number of operations performed.",
	}, []string{"poolname", "vdevtype", "vdevname", "vdevoptype"})

	vdevbytes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "zfs",
		Subsystem: "zpool",
		Name:      "vdevbytes_total",
		Help:      "number of bytes handled",
	}, []string{"poolname", "vdevtype", "vdevname", "vdevoptype"})

	vdeverrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "zfs",
		Subsystem: "zpool",
		Name:      "errors_total",
		Help:      "number of errors seen",
	}, []string{"poolname", "vdevtype", "vdevname", "errortype"})

	vdevstate = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "zfs",
		Subsystem: "zpool",
		Name:      "vdevstate",
		Help:      "vdev state: Unknown, Closed, Offline, Removed, CantOpen, Faulted, Degraded, Healthy.",
	}, []string{"poolname", "vdevtype", "vdevname"})

	vdevalloc = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "zfs",
		Subsystem: "zpool",
		Name:      "allocated_bytes",
		Help:      "number of bytes allocated (usage)",
	}, []string{"poolname", "vdevtype", "vdevname"})

	vdevspace = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "zfs",
		Subsystem: "zpool",
		Name:      "space_bytes",
		Help:      "size of the vdev in bytes (total capacity).",
	}, []string{"poolname", "vdevtype", "vdevname"})

	vdevfrag = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "zfs",
		Subsystem: "zpool",
		Name:      "fragmentation_percent",
		Help:      "device fragmentation percentage",
	}, []string{"poolname", "vdevtype", "vdevname"})

	poolstate = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "zfs",
		Subsystem: "zpool",
		Name:      "poolstate",
		Help:      "pool state enum: Active, Exported, Destroyed, Spare, L2cache, uninitialized, unavail, potentiallyactive",
	}, []string{"poolname"})

	poolstatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "zfs",
		Subsystem: "zpool",
		Name:      "poolstatus",
		Help:      "pool status enum: CorruptCache, MissingDevR, MissingDevNr, CorruptLabelR, CorruptLabelNr, BadGUIDSum, CorruptPool, CorruptData, FailingDev, VersionNewer, HostidMismatch, IoFailureWait, IoFailureContinue, BadLog, Errata, UnsupFeatRead, UnsupFeatWrite, FaultedDevR, FaultedDevNr, VersionOlder, FeatDisabled, Resilvering, OfflineDev, RemovedDev, Ok",
	}, []string{"poolname"})
)

func init() {
	prometheus.MustRegister(vdevops)
	prometheus.MustRegister(vdevbytes)
	prometheus.MustRegister(vdeverrors)
	prometheus.MustRegister(vdevstate)
	prometheus.MustRegister(vdevalloc)
	prometheus.MustRegister(vdevspace)
	prometheus.MustRegister(vdevfrag)
	prometheus.MustRegister(poolstate)
	prometheus.MustRegister(poolstatus)
}

func main() {
	pools, err := zfs.PoolOpenAll()
	if err != nil {
		log.Fatal("error opening pools: %v", err)
	}
	go serve(pools)

	http.Handle("/metrics", prometheus.Handler())
	http.ListenAndServe(":9054", nil)
}

func serve(pools []zfs.Pool) {
	for {
		for _, pool := range pools {
			poolstats(pool)
		}
		time.Sleep(1 * time.Second)
	}
	// TODO should we worry about shutting down gracefully and calling pool.Close()?
}

func poolname(pool zfs.Pool) string {
	return pool.Properties[zfs.PoolPropName].Value
}

func poolstats(pool zfs.Pool) {
	pool.RefreshStats()

	poolName := poolname(pool)
	pstatus, err := pool.Status()
	if err != nil {
		log.Printf("error getting status of pool '%s': %v\n", poolName, err)
		pstatus = -1
	}
	poolstatus.WithLabelValues(poolName).Set(float64(pstatus))

	pstate, err := pool.State()
	pstateFloat := float64(pstate)
	if err != nil {
		log.Printf("error getting state of pool '%s': %v\n", poolName, err)
		pstateFloat = -1
	}
	poolstate.WithLabelValues(poolName).Set(pstateFloat)

	vdevStats(pool)
}

func vdevStats(pool zfs.Pool) {
	vdt, err := pool.VDevTree()
	if err != nil {
		log.Printf("unable to read vdevtree for pool '%s': %v", poolname(pool), err)
	}
	// printpooltree(pool, vdt)
	visitVdevs(pool, vdt, vdevCollector)
}

func visitVdevs(pool zfs.Pool, vdt zfs.VDevTree, visitor func(pool zfs.Pool, vdt zfs.VDevTree)) {
	visitor(pool, vdt)
	for _, child := range vdt.Devices {
		visitVdevs(pool, child, visitor)
	}
}

func vdevCollector(pool zfs.Pool, vdt zfs.VDevTree) {
	poolName := poolname(pool)
	vType := string(vdt.Type)

	vdevstate.WithLabelValues(poolName, vType, vdt.Name).Set(float64(vdt.Stat.State))
	vdevalloc.WithLabelValues(poolName, vType, vdt.Name).Set(float64(vdt.Stat.Alloc))
	vdevspace.WithLabelValues(poolName, vType, vdt.Name).Set(float64(vdt.Stat.Space))
	vdevfrag.WithLabelValues(poolName, vType, vdt.Name).Set(float64(vdt.Stat.Fragmentation))

	vdeverrors.WithLabelValues(poolName, vType, vdt.Name, "read").Set(float64(vdt.Stat.ReadErrors))
	vdeverrors.WithLabelValues(poolName, vType, vdt.Name, "write").Set(float64(vdt.Stat.WriteErrors))
	vdeverrors.WithLabelValues(poolName, vType, vdt.Name, "checksum").Set(float64(vdt.Stat.ChecksumErrors))

	for optype := zfs.ZIOTypeRead; optype < zfs.ZIOTypes; optype++ {
		vdevops.WithLabelValues(poolName, vType, vdt.Name, zioTypeNames[optype]).Set(float64(vdt.Stat.Ops[optype]))
	}

	for optype := zfs.ZIOTypeRead; optype < zfs.ZIOTypes; optype++ {
		vdevbytes.WithLabelValues(poolName, vType, vdt.Name, zioTypeNames[optype]).Set(float64(vdt.Stat.Bytes[optype]))
	}
}
