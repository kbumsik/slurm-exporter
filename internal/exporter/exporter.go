// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package exporter

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log"

	api "github.com/SlinkyProject/slurm-client/api/v0041"
	"github.com/SlinkyProject/slurm-client/pkg/client"
	"github.com/SlinkyProject/slurm-client/pkg/object"
	slurmtypes "github.com/SlinkyProject/slurm-client/pkg/types"
)

var (
	partitionLabel = []string{"partition"}
	nodeLabel      = []string{"node"}
	jobLabel       = []string{"user"}
	jobInfoLabels  = []string{"id", "name", "state", "user", "node"}
	nodeRangeRegex = regexp.MustCompile(`(.*?)\[([0-9,\-]+)\](.*)`)
)

type PartitionData struct {
	Nodes           int32
	Cpus            int32
	PendingJobs     int32
	PendingMaxNodes int32
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
	slurmClient    client.Client
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
	nodeJobInfo              *prometheus.Desc
}

// Initialize the slurm client to talk to slurmrestd.
// Requires that the env SLURM_JWT is set.
func NewSlurmClient(server string, cacheFreq time.Duration) (client.Client, error) {
	ctx := context.Background()
	log := log.FromContext(ctx)

	token, ok := os.LookupEnv("SLURM_JWT")
	if !ok || token == "" {
		return nil, errors.New("SLURM_JWT must be defined and not empty")
	}

	// Create slurm client
	config := &client.Config{
		Server:    server,
		AuthToken: token,
	}

	// Instruct the client to keep a cache of slurm objects
	clientOptions := client.ClientOptions{
		EnableFor: []object.Object{
			&slurmtypes.V0041PartitionInfo{},
			&slurmtypes.V0041Node{},
			&slurmtypes.V0041JobInfo{},
		},
		CacheSyncPeriod: cacheFreq,
	}
	slurmClient, err := client.NewClient(config, &clientOptions)
	if err != nil {
		return nil, err
	}

	// Start client cache
	go slurmClient.Start(ctx)

	log.Info("Created slurm client")

	return slurmClient, nil
}

// slurmCollect will read slurm objects from slurmrestd and return slurmData to represent
// partition, node, and job information of the running slurm cluster
func (r *SlurmCollector) slurmCollectType(list object.ObjectList) error {
	ctx := context.Background()
	log := log.FromContext(ctx)

	// Read slurm information from cache
	if err := r.slurmClient.List(ctx, list); err != nil {
		log.Error(err, "Could not list objects")
		return err
	}

	return nil
}

// slurmParse will return slurmData to represent partition, node, and job
// information of the running slurm cluster.
func (r *SlurmCollector) slurmParse(
	jobs *slurmtypes.V0041JobInfoList,
	nodes *slurmtypes.V0041NodeList,
	partitions *slurmtypes.V0041PartitionInfoList,
) slurmData {

	ctx := context.Background()
	log := log.FromContext(ctx)

	slurmData := slurmData{}
	ns := NodeStates{}

	// Populate partitionData indexed by partition with the number of cpus
	// and nodes.
	partitionData := make(map[string]*PartitionData)
	for _, p := range partitions.Items {
		key := string(p.GetKey())
		_, ok := partitionData[key]
		if !ok {
			partitionData[key] = &PartitionData{}
		}
		partitionData[key].Cpus = ptr.Deref(p.Cpus.Total, 0)
		partitionData[key].Nodes = ptr.Deref(p.Nodes.Total, 0)
	}

	// Populate nodeData indexed by node with the number of cpus
	// and how many are allocated/idle.
	nodeData := make(map[string]*NodeData)
	for _, n := range nodes.Items {
		key := string(n.GetKey())
		_, ok := nodeData[key]
		if !ok {
			nodeData[key] = &NodeData{}
		}
		nodeData[key].Cpus = ptr.Deref(n.Cpus, 0)
		nodeData[key].Alloc = ptr.Deref(n.AllocCpus, 0)
		nodeData[key].Idle = ptr.Deref(n.AllocIdleCpus, 0)

		for _, s := range n.GetStateAsSet().UnsortedList() {
			switch s {
			case api.V0041NodeStateALLOCATED:
				ns.allocated++
			case api.V0041NodeStateCOMPLETING:
				ns.completing++
			case api.V0041NodeStateDOWN:
				ns.down++
			case api.V0041NodeStateDRAIN:
				ns.drain++
			case api.V0041NodeStateERROR:
				ns.err++
			case api.V0041NodeStateIDLE:
				ns.idle++
			case api.V0041NodeStateMAINTENANCE:
				ns.maintenance++
			case api.V0041NodeStateMIXED:
				ns.mixed++
			case api.V0041NodeStateRESERVED:
				ns.reserved++
			}
		}

		// Update partitionData with per node information as this
		// is not yet available with the /partitions endpoint.
		partitions := ptr.Deref(n.Partitions, api.V0041CsvString{})
		for _, p := range partitions {
			_, ok := partitionData[p]
			if ok {
				partitionData[p].Alloc += nodeData[key].Alloc
				partitionData[p].Idle += nodeData[key].Idle
			}
		}
	}

	// Populate jobData indexed by user with the number of jobs
	// they have and their various states.
	jobData := make(map[string]*JobData)
	for _, j := range jobs.Items {
		userName := ptr.Deref(j.UserName, "")
		partition := ptr.Deref(j.Partition, "")
		if _, key := jobData[userName]; !key {
			jobData[userName] = &JobData{}
		}
		if _, key := partitionData[partition]; !key {
			log.Info("No partition found for job")
			continue
		}
		// Update partitionData with the number of jobs scheduled
		if !j.GetStateAsSet().HasAny(
			api.V0041JobInfoJobStateCOMPLETED,
			api.V0041JobInfoJobStateCOMPLETING,
			api.V0041JobInfoJobStateCANCELLED) {
			partitionData[partition].Jobs++
			jobData[userName].Count++
		}
		isHold := ptr.Deref(j.Hold, false)
		if j.GetStateAsSet().HasAll(api.V0041JobInfoJobStatePENDING) &&
			!isHold {
			partitionData[partition].PendingJobs++
			jobData[userName].Pending++
		}
		if j.GetStateAsSet().HasAll(api.V0041JobInfoJobStateRUNNING) {
			partitionData[partition].RunningJobs++
			jobData[userName].Running++
		}
		if isHold {
			partitionData[partition].HoldJobs++
			jobData[userName].Hold++
		}
		// Track total pending nodes for a partition for jobs that
		// meet the criterea below. This exposes metrics that may be
		// used for autoscaling. Jobs with an eligible date greater
		// than one year are ignored.
		eligibleTimeNoVal := ptr.Deref(j.EligibleTime, api.V0041Uint64NoValStruct{})
		eligibleTime := ptr.Deref(eligibleTimeNoVal.Number, 0)
		if j.GetStateAsSet().HasAll(api.V0041JobInfoJobStatePENDING) &&
			eligibleTime >= 0 &&
			time.Unix(eligibleTime, 0).After(time.Now().AddDate(-1, 0, 0)) &&
			!isHold {
			nodeCountNoVal := ptr.Deref(j.NodeCount, api.V0041Uint32NoValStruct{})
			nodeCount := ptr.Deref(nodeCountNoVal.Number, 0)
			if partitionData[partition].PendingMaxNodes < nodeCount {
				partitionData[partition].PendingMaxNodes = nodeCount
			}
		}
	}

	slurmData.partitions = partitionData
	slurmData.nodes = nodeData
	slurmData.jobs = jobData
	slurmData.nodestates = ns
	return slurmData
}

func NewSlurmCollector(slurmClient client.Client, perUserMetrics bool) *SlurmCollector {
	return &SlurmCollector{
		slurmClient:              slurmClient,
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
		nodeJobInfo:              prometheus.NewDesc("slurm_node_job_info", "Information about a slurm job in a node", jobInfoLabels, nil),
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

// parseNodeRange expands a Slurm node specification string which might include
// comma-separated nodes and complex bracket expressions like "compute-[1,3-5],login-[01-02],node7"
// into a list of individual node names.
// It prioritizes finding and expanding bracket expressions first.
func parseNodeRange(nodeStr string) ([]string, error) {
	trimmedNodeStr := strings.TrimSpace(nodeStr)
	if trimmedNodeStr == "" {
		// Treat empty string and whitespace-only string as invalid input.
		return nil, fmt.Errorf("input node string is empty or consists only of whitespace: %q", nodeStr)
	}
	// Use trimmed string for processing, keep original for error messages
	processingStr := trimmedNodeStr

	var expandedNodes []string
	remainingStr := processingStr // Start processing with the trimmed string

	for {
		match := nodeRangeRegex.FindStringSubmatch(remainingStr)
		if len(match) != 4 {
			break // No more bracket expressions found in the remainder
		}

		preMatch := match[1] // String before the current bracket expression
		innerContent := match[2]
		suffix := match[3] // String after the current bracket expression

		// --- NEW CHECK for trailing characters after brackets ---
		// Valid suffixes must either be empty or start with a comma.
		trimmedSuffix := strings.TrimSpace(suffix)
		if trimmedSuffix != "" && !strings.HasPrefix(trimmedSuffix, ",") {
			bracketPart := "[" + innerContent + "]"
			return nil, fmt.Errorf("invalid node string format: trailing characters %q found immediately after bracket expression %q in original input %q", trimmedSuffix, bracketPart, nodeStr)
		}
		// --- END NEW CHECK ---

		prefix := preMatch // The actual prefix for the nodes generated from this bracket (potentially updated below)

		// Process the part *before* the current bracket match
		if strings.Contains(preMatch, ",") {
			// If the pre-match part contains commas, split it and treat
			// the last segment as the prefix for the current bracket group.
			preParts := strings.Split(preMatch, ",")
			for i := 0; i < len(preParts)-1; i++ {
				node := strings.TrimSpace(preParts[i])
				if node != "" {
					if strings.ContainsAny(node, "[]") {
						return nil, fmt.Errorf("unexpected brackets in node name segment %q from %q", node, nodeStr)
					}
					expandedNodes = append(expandedNodes, node)
				}
			}
			prefix = strings.TrimSpace(preParts[len(preParts)-1])
		} else {
			// No comma in pre-match, the whole thing is the prefix
			prefix = strings.TrimSpace(preMatch)
		}

		// --- Process the bracketed content [innerContent] ---
		innerParts := strings.Split(innerContent, ",")
		maxNumLen := 0 // Track max length for padding within this bracket group

		// First pass over inner parts to determine padding
		for _, innerPart := range innerParts {
			innerPart = strings.TrimSpace(innerPart)
			if innerPart == "" {
				continue
			}
			if strings.Contains(innerPart, "-") {
				rangeSplit := strings.SplitN(innerPart, "-", 2)
				if len(rangeSplit) == 2 {
					startStr := strings.TrimSpace(rangeSplit[0])
					endStr := strings.TrimSpace(rangeSplit[1])
					if _, err := strconv.Atoi(startStr); err != nil {
						return nil, fmt.Errorf("invalid start range number %q in %q: %w", startStr, innerContent, err)
					}
					if _, err := strconv.Atoi(endStr); err != nil {
						return nil, fmt.Errorf("invalid end range number %q in %q: %w", endStr, innerContent, err)
					}
					if len(endStr) > maxNumLen {
						maxNumLen = len(endStr)
					}
					if len(startStr) > maxNumLen {
						maxNumLen = len(startStr)
					}
				} else {
					return nil, fmt.Errorf("invalid range format %q in %q", innerPart, innerContent)
				}
			} else { // Single number
				if _, err := strconv.Atoi(innerPart); err != nil {
					return nil, fmt.Errorf("invalid number %q in %q: %w", innerPart, innerContent, err)
				}
				if len(innerPart) > maxNumLen {
					maxNumLen = len(innerPart)
				}
			}
		}

		if maxNumLen == 0 {
			maxNumLen = 1
		}
		format := fmt.Sprintf("%%s%%0%dd", maxNumLen) // e.g., "%s%02d", suffix added later

		// Second pass to generate nodes for this bracket group
		for _, innerPart := range innerParts {
			innerPart = strings.TrimSpace(innerPart)
			if innerPart == "" {
				continue
			}
			if strings.Contains(innerPart, "-") {
				rangeSplit := strings.SplitN(innerPart, "-", 2)
				startStr := strings.TrimSpace(rangeSplit[0])
				endStr := strings.TrimSpace(rangeSplit[1])
				start, _ := strconv.Atoi(startStr)
				end, _ := strconv.Atoi(endStr)

				if start > end {
					return nil, fmt.Errorf("start range %d greater than end range %d in %q", start, end, innerContent)
				}

				for i := start; i <= end; i++ {
					// Suffix is handled below when processing remainingStr
					expandedNodes = append(expandedNodes, fmt.Sprintf(format, prefix, i))
				}
			} else { // Single number
				num, _ := strconv.Atoi(innerPart)
				expandedNodes = append(expandedNodes, fmt.Sprintf(format, prefix, num))
			}
		}
		// --- End bracket processing ---

		// Continue processing with the part *after* the current bracket match
		remainingStr = suffix // Use the original suffix (not trimmed) for the next loop iteration
	}

	// Process the final remaining part of the string (contains no brackets)
	if remainingStr != "" {
		finalParts := strings.Split(remainingStr, ",")
		for _, node := range finalParts {
			node = strings.TrimSpace(node)
			if node != "" {
				if strings.ContainsAny(node, "[]") {
					return nil, fmt.Errorf("unexpected brackets in final node name segment %q from %q", node, nodeStr)
				}
				expandedNodes = append(expandedNodes, node)
			}
		}
	}

	// If after all processing, the list is still empty but the input wasn't just whitespace, return an error.
	if len(expandedNodes) == 0 {
		// Note: processingStr is the trimmed version.
		// We already handled the case where the original string was purely whitespace or empty.
		// So if we reach here with no nodes, it means the input had content but it was invalid (e.g., ",,").
		return nil, fmt.Errorf("input node string %q resulted in no valid node names", nodeStr) // Use original in error
	}

	return expandedNodes, nil
}

// Called by the Prometheus registry when collecting metrics.
// https://pkg.go.dev/github.com/prometheus/client_golang/prometheus#Collector
func (s *SlurmCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()
	log := log.FromContext(ctx)
	log.Info("Collecting slurm data.")

	// Read slurm information from cache
	nodes := &slurmtypes.V0041NodeList{}
	if err := s.slurmCollectType(nodes); err != nil {
		log.Error(err, "Could not list nodes")
		return
	}
	jobs := &slurmtypes.V0041JobInfoList{}
	if err := s.slurmCollectType(jobs); err != nil {
		log.Error(err, "Could not list jobs")
		return
	}
	partitions := &slurmtypes.V0041PartitionInfoList{}
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
	for _, j := range jobs.Items {
		jobID := ptr.Deref(j.JobId, int32(0))
		jobName := ptr.Deref(j.Name, "")
		jobState := ""
		if len(j.GetStateAsSet().UnsortedList()) > 0 {
			jobState = string(j.GetStateAsSet().UnsortedList()[0])
		}
		userName := ptr.Deref(j.UserName, "")
		jobNodeRaw := ptr.Deref(j.Nodes, "")

		expandedNodes, err := parseNodeRange(jobNodeRaw)
		if err != nil {
			log.Error(err, "Failed to parse node range", "jobId", jobID, "nodeString", jobNodeRaw)
			// Send metric with the original raw string if parsing fails
			expandedNodes = []string{jobNodeRaw}
		}

		// Handle case where parseNodeRange returns empty for an empty input
		if len(expandedNodes) == 0 && jobNodeRaw == "" {
			expandedNodes = []string{""} // Ensure at least one metric is sent for jobs without nodes
		}

		for _, node := range expandedNodes {
			ch <- prometheus.MustNewConstMetric(s.nodeJobInfo, prometheus.GaugeValue, 1,
				fmt.Sprintf("%d", jobID), jobName, jobState, userName, node)
		}
	}
}
