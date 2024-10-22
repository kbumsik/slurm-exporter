// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package exporter

import (
	"k8s.io/utils/set"

	slurmtypes "github.com/SlinkyProject/slurm-client/pkg/types"
)

var (
	jobStateSetPending    = make(set.Set[slurmtypes.JobInfoJobState]).Insert(slurmtypes.JobInfoJobStatePENDING)
	jobStateSetRunning    = make(set.Set[slurmtypes.JobInfoJobState]).Insert(slurmtypes.JobInfoJobStateRUNNING)
	partitionStateSetUp   = make(set.Set[slurmtypes.PartitionInfoPartitionState]).Insert(slurmtypes.PartitionInfoPartitionStateUP)
	partitionStateSetDown = make(set.Set[slurmtypes.PartitionInfoPartitionState]).Insert(slurmtypes.PartitionInfoPartitionStateDOWN)
	nodeStateSetIdle      = make(set.Set[slurmtypes.NodeState]).Insert(slurmtypes.NodeStateIDLE)
	nodeStateSetAllocated = make(set.Set[slurmtypes.NodeState]).Insert(slurmtypes.NodeStateALLOCATED)
	// nodeStateSetMisc is not a valid Slurm state for a node. It is used during testing to verify a NodeStates struct.
	nodeStateSetMisc = make(set.Set[slurmtypes.NodeState]).Insert(slurmtypes.NodeStateCOMPLETING,
		slurmtypes.NodeStateDOWN,
		slurmtypes.NodeStateDRAIN,
		slurmtypes.NodeStateERROR,
		slurmtypes.NodeStateMAINTENANCE,
		slurmtypes.NodeStateMIXED,
		slurmtypes.NodeStateRESERVED)
	nodePartitionSetPurple      = make(set.Set[string]).Insert("purple")
	nodePartitionSetGreen       = make(set.Set[string]).Insert("green")
	nodePartitionSetPurpleGreen = make(set.Set[string]).Insert("purple").Insert("green")

	jobInfoA   = &slurmtypes.JobInfo{JobId: 1, UserName: "slurm", Partition: "purple", JobState: jobStateSetPending, Cpus: 12, NodeCount: 1, Hold: false, EligibleTime: 1722609649}
	jobInfoB   = &slurmtypes.JobInfo{JobId: 2, UserName: "slurm", Partition: "green", JobState: jobStateSetRunning, Cpus: 12, NodeCount: 1, Hold: true, EligibleTime: 1722609649}
	jobInfoC   = &slurmtypes.JobInfo{JobId: 3, UserName: "slurm", Partition: "foo", JobState: jobStateSetPending, Cpus: 12, NodeCount: 2, Hold: false, EligibleTime: 0}
	jobInfoD   = &slurmtypes.JobInfo{JobId: 4, UserName: "hal", Partition: "green", JobState: jobStateSetPending, Cpus: 12, NodeCount: 2, Hold: false, EligibleTime: 0}
	partitionA = &slurmtypes.PartitionInfo{Name: "purple", NodeSets: "purple", Nodes: 2, Cpus: 20, PartitionState: partitionStateSetUp}
	partitionB = &slurmtypes.PartitionInfo{Name: "green", NodeSets: "green", Nodes: 2, Cpus: 32, PartitionState: partitionStateSetDown}
	nodeA      = &slurmtypes.Node{Name: "kind-worker", Address: "10.244.1.1", Partitions: nodePartitionSetPurple, State: nodeStateSetIdle, Cpus: 12, AllocCpus: 0, AllocIdleCpus: 12}
	nodeB      = &slurmtypes.Node{Name: "kind-worker2", Address: "10.244.1.2", Partitions: nodePartitionSetGreen, State: nodeStateSetAllocated, Cpus: 24, AllocCpus: 12, AllocIdleCpus: 12}
	nodeC      = &slurmtypes.Node{Name: "kind-worker3", Address: "10.244.1.3", Partitions: nodePartitionSetPurpleGreen, State: nodeStateSetMisc, Cpus: 8, AllocCpus: 4, AllocIdleCpus: 4}
)
