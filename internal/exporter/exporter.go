// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package exporter

import (
	"context"
	"errors"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/SlinkyProject/slurm-client/pkg/client"
	"github.com/SlinkyProject/slurm-client/pkg/object"
	slurmtypes "github.com/SlinkyProject/slurm-client/pkg/types"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/SlinkyProject/slurm-exporter/internal/resources"
)

var (
	partitionLabel = []string{"partition"}
	nodeLabel      = []string{"node"}
	jobLabel       = []string{"user"}
)

type PartitionData struct {
	Nodes           int32
	Cpus            int32
	PendingJobs     int32
	PendingMaxNodes int64
	RunningJobs     int32
	HoldJobs        int32
	Jobs            int32
	Alloc           int32
	Idle            int32
}

type NodeData struct {
	Cpus  int32
	Alloc int32
	Idle  int32
}

type NodeStates struct {
	allocated   int32
	completing  int32
	down        int32
	drain       int32
	err         int32
	idle        int32
	maintenance int32
	mixed       int32
	reserved    int32
}

type JobData struct {
	Count   int32
	Pending int32
	Running int32
	Hold    int32
}

type slurmData struct {
	partitions map[string]*PartitionData
	nodes      map[string]*NodeData
	nodestates NodeStates
	jobs       map[string]*JobData
}

type SlurmCollector struct {
	slurmClusters  *resources.Clusters
	name           types.NamespacedName
	server         string
	cacheFreq      time.Duration
	perUserMetrics bool

	partitionNodes           *prometheus.Desc
	partitionCpus            *prometheus.Desc
	partitionIdleCpus        *prometheus.Desc
	partitionAllocCpus       *prometheus.Desc
	partitionJobs            *prometheus.Desc
	partitionPendingJobs     *prometheus.Desc
	partitionMaxPendingNodes *prometheus.Desc
	partitionRunningJobs     *prometheus.Desc
	partitionHoldJobs        *prometheus.Desc
	nodes                    *prometheus.Desc
	nodeCpus                 *prometheus.Desc
	nodeIdleCpus             *prometheus.Desc
	nodeAllocCpus            *prometheus.Desc
	allocNodes               *prometheus.Desc
	completingNodes          *prometheus.Desc
	downNodes                *prometheus.Desc
	drainNodes               *prometheus.Desc
	errNodes                 *prometheus.Desc
	idleNodes                *prometheus.Desc
	maintenanceNodes         *prometheus.Desc
	mixedNodes               *prometheus.Desc
	reservedNodes            *prometheus.Desc
	userJobTotal             *prometheus.Desc
	userPendingJobs          *prometheus.Desc
	userRunningJobs          *prometheus.Desc
	userHoldJobs             *prometheus.Desc
}

// Initialize the slurm client to talk to slurmrestd.
// Requires that the env METRICS_TOKEN is set.
func (r *SlurmCollector) SlurmClient() error {
	ctx := context.Background()
	log := log.FromContext(ctx)

	token, ok := os.LookupEnv("METRICS_TOKEN")
	if !ok || token == "" {
		return errors.New("METRICS_TOKEN must be defined and not empty")
	}

	// Create slurm client
	config := &client.Config{
		Server:    r.server,
		AuthToken: token,
	}

	// Instruct the client to keep a cache of slurm objects
	clientOptions := client.ClientOptions{
		EnableFor: []object.Object{
			&slurmtypes.PartitionInfo{},
			&slurmtypes.Node{},
			&slurmtypes.JobInfo{},
		},
		CacheSyncPeriod: r.cacheFreq,
	}
	slurmClient, err := client.NewClient(config, &clientOptions)
	if err != nil {
		return err
	}

	// Add slurm client
	r.slurmClusters.Add(r.name, slurmClient)
	log.Info("Added slurm cluster client", "clusterName", r.name)

	return nil
}

// slurmCollect will read slurm objects from slurmrestd and return slurmData to represent
// partition, node, and job information of the running slurm cluster
func (r *SlurmCollector) slurmCollectType(list object.ObjectList) error {
	ctx := context.Background()
	log := log.FromContext(ctx)

	// Read slurm information from cache
	slurmCluster := r.slurmClusters.Get(r.name)
	err := slurmCluster.List(ctx, list)
	if err != nil {
		log.Error(err, "Could not list objects")
		return err
	}

	return nil
}

// slurmParse will return slurmData to represent partition, node, and job
// information of the running slurm cluster.
func (r *SlurmCollector) slurmParse(
	jobs *slurmtypes.JobInfoList,
	nodes *slurmtypes.NodeList,
	partitions *slurmtypes.PartitionInfoList) slurmData {

	ctx := context.Background()
	log := log.FromContext(ctx)

	slurmData := slurmData{}
	ns := NodeStates{}

	// Populate partitionData indexed by partition with the number of cpus
	// and nodes.
	partitionData := make(map[string]*PartitionData)
	for _, p := range partitions.Items {
		_, key := partitionData[p.Name]
		if !key {
			partitionData[p.Name] = &PartitionData{}
		}
		partitionData[p.Name].Cpus = p.Cpus
		partitionData[p.Name].Nodes = p.Nodes
	}

	// Populate nodeData indexed by node with the number of cpus
	// and how many are allocated/idle.
	nodeData := make(map[string]*NodeData)
	for _, n := range nodes.Items {
		_, key := nodeData[n.Name]
		if !key {
			nodeData[n.Name] = &NodeData{}
		}
		nodeData[n.Name].Cpus = n.Cpus
		nodeData[n.Name].Alloc = n.AllocCpus
		nodeData[n.Name].Idle = n.AllocIdleCpus

		for _, s := range n.State.UnsortedList() {
			switch s {
			case slurmtypes.NodeStateALLOCATED:
				ns.allocated++
			case slurmtypes.NodeStateCOMPLETING:
				ns.completing++
			case slurmtypes.NodeStateDOWN:
				ns.down++
			case slurmtypes.NodeStateDRAIN:
				ns.drain++
			case slurmtypes.NodeStateERROR:
				ns.err++
			case slurmtypes.NodeStateIDLE:
				ns.idle++
			case slurmtypes.NodeStateMAINTENANCE:
				ns.maintenance++
			case slurmtypes.NodeStateMIXED:
				ns.mixed++
			case slurmtypes.NodeStateRESERVED:
				ns.reserved++
			}
		}

		// Update partitionData with per node information as this
		// is not yet available with the /partitions endpoint.
		for _, p := range n.Partitions.UnsortedList() {
			_, key := partitionData[p]
			if key {
				partitionData[p].Alloc += nodeData[n.Name].Alloc
				partitionData[p].Idle += nodeData[n.Name].Idle
			}
		}
	}

	// Populate jobData indexed by user with the number of jobs
	// they have and their various states.
	jobData := make(map[string]*JobData)
	for _, j := range jobs.Items {

		if _, key := jobData[j.UserName]; !key {
			jobData[j.UserName] = &JobData{}
		}
		if _, key := partitionData[j.Partition]; !key {
			log.Info("No partition found for job")
			continue
		}
		// Update partitionData with the number of jobs scheduled
		if !j.JobState.HasAny(
			slurmtypes.JobInfoJobStateCOMPLETED,
			slurmtypes.JobInfoJobStateCOMPLETING,
			slurmtypes.JobInfoJobStateCANCELLED) {
			partitionData[j.Partition].Jobs++
			jobData[j.UserName].Count++
		}
		if j.JobState.HasAll(slurmtypes.JobInfoJobStatePENDING) &&
			!j.Hold {
			partitionData[j.Partition].PendingJobs++
			jobData[j.UserName].Pending++
		}
		if j.JobState.HasAll(slurmtypes.JobInfoJobStateRUNNING) {
			partitionData[j.Partition].RunningJobs++
			jobData[j.UserName].Running++
		}
		if j.Hold {
			partitionData[j.Partition].HoldJobs++
			jobData[j.UserName].Hold++
		}
		// Track total pending nodes for a partition for jobs that
		// meet the criterea below. This exposes metrics that may be
		// used for autoscaling. Jobs with an eligible date greater
		// than one year are ignored.
		if j.JobState.HasAll(slurmtypes.JobInfoJobStatePENDING) &&
			j.EligibleTime >= 0 &&
			time.Unix(j.EligibleTime, 0).After(time.Now().AddDate(-1, 0, 0)) &&
			!j.Hold {
			if partitionData[j.Partition].PendingMaxNodes < j.NodeCount {
				partitionData[j.Partition].PendingMaxNodes = j.NodeCount
			}
		}
	}

	slurmData.partitions = partitionData
	slurmData.nodes = nodeData
	slurmData.jobs = jobData
	slurmData.nodestates = ns
	return slurmData
}

func NewSlurmCollector(
	name types.NamespacedName,
	server string,
	cacheFreq time.Duration,
	perUserMetrics bool,
) *SlurmCollector {
	return &SlurmCollector{
		slurmClusters:            resources.NewClusters(),
		server:                   server,
		name:                     name,
		cacheFreq:                cacheFreq,
		perUserMetrics:           perUserMetrics,
		partitionNodes:           prometheus.NewDesc("slurm_partition_nodes", "Number of nodes in a slurm partition", partitionLabel, nil),
		partitionCpus:            prometheus.NewDesc("slurm_partition_cpus", "Number of CPUs in a slurm partition", partitionLabel, nil),
		partitionIdleCpus:        prometheus.NewDesc("slurm_partition_idle_cpus", "Number of idle CPUs in a slurm partition", partitionLabel, nil),
		partitionAllocCpus:       prometheus.NewDesc("slurm_partition_alloc_cpus", "Number of allocated CPUs in a slurm partition", partitionLabel, nil),
		partitionJobs:            prometheus.NewDesc("slurm_partition_jobs", "Number of allocated CPUs in a slurm partition", partitionLabel, nil),
		partitionPendingJobs:     prometheus.NewDesc("slurm_partition_pending_jobs", "Number of pending jobs in a slurm partition", partitionLabel, nil),
		partitionMaxPendingNodes: prometheus.NewDesc("slurm_partition_max_pending_nodes", "Number of nodes pending for the largest job in the partition", partitionLabel, nil),
		partitionRunningJobs:     prometheus.NewDesc("slurm_partition_running_jobs", "Number of runnning jobs in a slurm partition", partitionLabel, nil),
		partitionHoldJobs:        prometheus.NewDesc("slurm_partition_hold_jobs", "Number of hold jobs in a slurm partition", partitionLabel, nil),
		nodes:                    prometheus.NewDesc("slurm_node_total", "Total number of slurm nodes", nil, nil),
		nodeCpus:                 prometheus.NewDesc("slurm_node_cpus", "Number of CPUs in a slurm node", nodeLabel, nil),
		nodeIdleCpus:             prometheus.NewDesc("slurm_node_idle_cpus", "Number of idle CPUs in a slurm node", nodeLabel, nil),
		nodeAllocCpus:            prometheus.NewDesc("slurm_node_alloc_cpus", "Number of allocated CPUs in a slurm node", nodeLabel, nil),
		allocNodes:               prometheus.NewDesc("slurm_alloc_nodes", "Number of nodes in allocated state", nil, nil),
		completingNodes:          prometheus.NewDesc("slurm_completing_nodes", "Number of nodes in completing state", nil, nil),
		downNodes:                prometheus.NewDesc("slurm_down_nodes", "Number of nodes in down state", nil, nil),
		drainNodes:               prometheus.NewDesc("slurm_drain_nodes", "Number of nodes in drain state", nil, nil),
		errNodes:                 prometheus.NewDesc("slurm_err_nodes", "Number of nodes in error state", nil, nil),
		idleNodes:                prometheus.NewDesc("slurm_idle_nodes", "Number of nodes in idle state", nil, nil),
		maintenanceNodes:         prometheus.NewDesc("slurm_maintenance_nodes", "Number of nodes in maintenance state", nil, nil),
		mixedNodes:               prometheus.NewDesc("slurm_mixed_nodes", "Number of nodes in mixed state", nil, nil),
		reservedNodes:            prometheus.NewDesc("slurm_reserved_nodes", "Number of nodes in reserved state", nil, nil),
		userJobTotal:             prometheus.NewDesc("slurm_job_total", "Number of jobs for a slurm user", jobLabel, nil),
		userPendingJobs:          prometheus.NewDesc("slurm_user_pending_jobs", "Number of pending jobs for a slurm user", jobLabel, nil),
		userRunningJobs:          prometheus.NewDesc("slurm_user_running_jobs", "Number of running jobs for a slurm user", jobLabel, nil),
		userHoldJobs:             prometheus.NewDesc("slurm_user_hold_jobs", "Number of hold jobs for a slurm user", jobLabel, nil),
	}
}

// Send all of the possible descriptions of the metrics to be collected by the Collector
// https://pkg.go.dev/github.com/prometheus/client_golang/prometheus#Collector
func (s *SlurmCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- s.partitionNodes
	ch <- s.partitionCpus
	ch <- s.partitionIdleCpus
	ch <- s.partitionAllocCpus
	ch <- s.partitionJobs
	ch <- s.partitionPendingJobs
	ch <- s.partitionMaxPendingNodes
	ch <- s.partitionRunningJobs
	ch <- s.partitionHoldJobs
	ch <- s.nodes
	ch <- s.nodeCpus
	ch <- s.nodeIdleCpus
	ch <- s.nodeAllocCpus
	ch <- s.allocNodes
	ch <- s.completingNodes
	ch <- s.downNodes
	ch <- s.drainNodes
	ch <- s.errNodes
	ch <- s.idleNodes
	ch <- s.maintenanceNodes
	ch <- s.mixedNodes
	ch <- s.reservedNodes
	ch <- s.userJobTotal
	ch <- s.userRunningJobs
	ch <- s.userPendingJobs
	ch <- s.userHoldJobs
}

// Called by the Prometheus registry when collecting metrics.
// https://pkg.go.dev/github.com/prometheus/client_golang/prometheus#Collector
func (s *SlurmCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()
	log := log.FromContext(ctx)
	log.Info("Collecting slurm data.")

	// Read slurm information from cache
	nodes := &slurmtypes.NodeList{}
	if err := s.slurmCollectType(nodes); err != nil {
		log.Error(err, "Could not list nodes")
		return
	}
	jobs := &slurmtypes.JobInfoList{}
	if err := s.slurmCollectType(jobs); err != nil {
		log.Error(err, "Could not list jobs")
		return
	}
	partitions := &slurmtypes.PartitionInfoList{}
	if err := s.slurmCollectType(partitions); err != nil {
		log.Error(err, "Could not list partitions")
		return
	}

	slurmData := s.slurmParse(jobs, nodes, partitions)

	for p := range slurmData.partitions {
		ch <- prometheus.MustNewConstMetric(s.partitionNodes, prometheus.GaugeValue, float64(slurmData.partitions[p].Nodes), p)
		ch <- prometheus.MustNewConstMetric(s.partitionCpus, prometheus.GaugeValue, float64(slurmData.partitions[p].Cpus), p)
		ch <- prometheus.MustNewConstMetric(s.partitionIdleCpus, prometheus.GaugeValue, float64(slurmData.partitions[p].Idle), p)
		ch <- prometheus.MustNewConstMetric(s.partitionAllocCpus, prometheus.GaugeValue, float64(slurmData.partitions[p].Alloc), p)
		ch <- prometheus.MustNewConstMetric(s.partitionJobs, prometheus.GaugeValue, float64(slurmData.partitions[p].Jobs), p)
		ch <- prometheus.MustNewConstMetric(s.partitionPendingJobs, prometheus.GaugeValue, float64(slurmData.partitions[p].PendingJobs), p)
		ch <- prometheus.MustNewConstMetric(s.partitionMaxPendingNodes, prometheus.GaugeValue, float64(slurmData.partitions[p].PendingMaxNodes), p)
		ch <- prometheus.MustNewConstMetric(s.partitionRunningJobs, prometheus.GaugeValue, float64(slurmData.partitions[p].RunningJobs), p)
		ch <- prometheus.MustNewConstMetric(s.partitionHoldJobs, prometheus.GaugeValue, float64(slurmData.partitions[p].HoldJobs), p)
	}
	ch <- prometheus.MustNewConstMetric(s.nodes, prometheus.GaugeValue, float64(len(slurmData.nodes)))
	for n := range slurmData.nodes {
		ch <- prometheus.MustNewConstMetric(s.nodeCpus, prometheus.GaugeValue, float64(slurmData.nodes[n].Cpus), n)
		ch <- prometheus.MustNewConstMetric(s.nodeIdleCpus, prometheus.GaugeValue, float64(slurmData.nodes[n].Idle), n)
		ch <- prometheus.MustNewConstMetric(s.nodeAllocCpus, prometheus.GaugeValue, float64(slurmData.nodes[n].Alloc), n)
	}
	ch <- prometheus.MustNewConstMetric(s.allocNodes, prometheus.GaugeValue, float64(slurmData.nodestates.allocated))
	ch <- prometheus.MustNewConstMetric(s.completingNodes, prometheus.GaugeValue, float64(slurmData.nodestates.completing))
	ch <- prometheus.MustNewConstMetric(s.downNodes, prometheus.GaugeValue, float64(slurmData.nodestates.down))
	ch <- prometheus.MustNewConstMetric(s.drainNodes, prometheus.GaugeValue, float64(slurmData.nodestates.drain))
	ch <- prometheus.MustNewConstMetric(s.errNodes, prometheus.GaugeValue, float64(slurmData.nodestates.err))
	ch <- prometheus.MustNewConstMetric(s.idleNodes, prometheus.GaugeValue, float64(slurmData.nodestates.idle))
	ch <- prometheus.MustNewConstMetric(s.maintenanceNodes, prometheus.GaugeValue, float64(slurmData.nodestates.maintenance))
	ch <- prometheus.MustNewConstMetric(s.mixedNodes, prometheus.GaugeValue, float64(slurmData.nodestates.mixed))
	ch <- prometheus.MustNewConstMetric(s.reservedNodes, prometheus.GaugeValue, float64(slurmData.nodestates.reserved))
	if s.perUserMetrics {
		for j := range slurmData.jobs {
			ch <- prometheus.MustNewConstMetric(s.userJobTotal, prometheus.GaugeValue, float64(slurmData.jobs[j].Count), j)
			ch <- prometheus.MustNewConstMetric(s.userPendingJobs, prometheus.GaugeValue, float64(slurmData.jobs[j].Pending), j)
			ch <- prometheus.MustNewConstMetric(s.userRunningJobs, prometheus.GaugeValue, float64(slurmData.jobs[j].Running), j)
			ch <- prometheus.MustNewConstMetric(s.userHoldJobs, prometheus.GaugeValue, float64(slurmData.jobs[j].Hold), j)
		}
	}
}
